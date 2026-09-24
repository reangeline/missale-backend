package storekit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

var now = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

type chain struct {
	root         *x509.Certificate
	leafKey      *ecdsa.PrivateKey
	leafDER      []byte
	intermediate []byte
}

// newChain builds root → intermediate → leaf carrying Apple's marker OIDs,
// so the verifier's chain and signature checks run for real.
func newChain(t *testing.T, markers bool) chain {
	t.Helper()
	mk := func(tmpl, parent *x509.Certificate, pub *ecdsa.PublicKey, signer *ecdsa.PrivateKey) (*x509.Certificate, []byte) {
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, signer)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := x509.ParseCertificate(der)
		return c, der
	}
	key := func() *ecdsa.PrivateKey { k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader); return k }
	ext := func(oid asn1.ObjectIdentifier) []pkix.Extension {
		if !markers {
			return nil
		}
		return []pkix.Extension{{Id: oid, Value: []byte{5, 0}}}
	}
	validity := func(serial int64, cn string) x509.Certificate {
		return x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour)}
	}

	rootKey, interKey, leafKey := key(), key(), key()
	rootT := validity(1, "Test Root")
	rootT.IsCA, rootT.BasicConstraintsValid, rootT.KeyUsage = true, true, x509.KeyUsageCertSign
	root, _ := mk(&rootT, &rootT, &rootKey.PublicKey, rootKey)

	interT := validity(2, "Test Intermediate")
	interT.IsCA, interT.BasicConstraintsValid, interT.KeyUsage = true, true, x509.KeyUsageCertSign
	interT.ExtraExtensions = ext(oidIntermediateMarker)
	inter, interDER := mk(&interT, root, &interKey.PublicKey, rootKey)

	leafT := validity(3, "Test Leaf")
	leafT.KeyUsage = x509.KeyUsageDigitalSignature
	leafT.ExtraExtensions = ext(oidLeafMarker)
	_, leafDER := mk(&leafT, inter, &leafKey.PublicKey, interKey)

	return chain{root: root, leafKey: leafKey, leafDER: leafDER, intermediate: interDER}
}

func (c chain) sign(t *testing.T, tx domain.SubscriptionTransaction) string {
	t.Helper()
	enc := func(v any) string { b, _ := json.Marshal(v); return base64.RawURLEncoding.EncodeToString(b) }
	header := enc(map[string]any{"alg": "ES256", "x5c": []string{
		base64.StdEncoding.EncodeToString(c.leafDER), base64.StdEncoding.EncodeToString(c.intermediate)}})
	input := header + "." + enc(tx)
	digest := sha256.Sum256([]byte(input))
	r, s, err := ecdsa.Sign(rand.Reader, c.leafKey, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return input + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func verifierTrusting(root *x509.Certificate) *verifier {
	pool := x509.NewCertPool()
	pool.AddCert(root)
	return &verifier{roots: pool, bundleID: "com.holymessages.app", productIDs: []string{"mensal", "anual"},
		now: func() time.Time { return now }}
}

func activeTx() domain.SubscriptionTransaction {
	return domain.SubscriptionTransaction{BundleID: "com.holymessages.app", ProductID: "anual", OriginalTransactionID: "1",
		ExpiresDate: now.Add(7 * 24 * time.Hour).UnixMilli(), Environment: "Sandbox"}
}

func TestVerifyAcceptsActiveSubscription(t *testing.T) {
	c := newChain(t, true)
	tx, err := verifierTrusting(c.root).Verify(c.sign(t, activeTx()))
	if err != nil {
		t.Fatalf("expected active subscription, got %v", err)
	}
	if tx.ProductID != "anual" {
		t.Fatalf("product = %q", tx.ProductID)
	}
}

func TestVerifyRejectsInactiveOrForeignTransactions(t *testing.T) {
	c := newChain(t, true)
	v := verifierTrusting(c.root)
	cases := map[string]func(*domain.SubscriptionTransaction){
		"expired":       func(tx *domain.SubscriptionTransaction) { tx.ExpiresDate = now.Add(-time.Minute).UnixMilli() },
		"revoked":       func(tx *domain.SubscriptionTransaction) { tx.RevocationDate = now.UnixMilli() },
		"other bundle":  func(tx *domain.SubscriptionTransaction) { tx.BundleID = "com.other.app" },
		"other product": func(tx *domain.SubscriptionTransaction) { tx.ProductID = "vitalicio" },
	}
	for name, mutate := range cases {
		tx := activeTx()
		mutate(&tx)
		if _, err := v.Verify(c.sign(t, tx)); !errors.Is(err, domain.ErrNotSubscribed) {
			t.Errorf("%s: expected domain.ErrNotSubscribed, got %v", name, err)
		}
	}
}

func TestVerifyRejectsForgeries(t *testing.T) {
	c := newChain(t, true)
	v := verifierTrusting(c.root)
	jws := c.sign(t, activeTx())

	// Payload edited after signing (e.g. expiry pushed forward).
	parts := strings.Split(jws, ".")
	late := activeTx()
	late.ExpiresDate = now.Add(365 * 24 * time.Hour).UnixMilli()
	b, _ := json.Marshal(late)
	if _, err := v.Verify(parts[0] + "." + base64.RawURLEncoding.EncodeToString(b) + "." + parts[2]); err == nil {
		t.Error("tampered payload accepted")
	}

	// Validly signed, but by a chain that doesn't lead to the trusted root.
	other := newChain(t, true)
	if _, err := v.Verify(other.sign(t, activeTx())); err == nil {
		t.Error("untrusted chain accepted")
	}

	// Trusted root, but certificates without Apple's StoreKit markers.
	plain := newChain(t, false)
	if _, err := verifierTrusting(plain.root).Verify(plain.sign(t, activeTx())); err == nil {
		t.Error("chain without StoreKit markers accepted")
	}

	for _, junk := range []string{"", "a.b", "not.a.jws"} {
		if _, err := v.Verify(junk); err == nil {
			t.Errorf("junk %q accepted", junk)
		}
	}
}

func TestEmbeddedAppleRootParses(t *testing.T) {
	if _, err := NewVerifier("com.holymessages.app", []string{"mensal"}); err != nil {
		t.Fatalf("Apple root CA G3: %v", err)
	}
}
