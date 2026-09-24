// Package storekit checks that a StoreKit 2 transaction, sent by the app
// as its signed JWS, is a live Missale subscription.
package storekit

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

//go:embed AppleRootCA-G3.cer
var appleRootG3 []byte

// Marker extensions Apple puts on the StoreKit signing chain.
var (
	oidLeafMarker         = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	oidIntermediateMarker = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}
)

type verifier struct {
	roots      *x509.CertPool
	bundleID   string
	productIDs []string
	now        func() time.Time
}

func NewVerifier(bundleID string, productIDs []string) (outbound.SubscriptionVerifier, error) {
	root, err := x509.ParseCertificate(appleRootG3)
	if err != nil {
		return nil, fmt.Errorf("Apple root: %w", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return &verifier{roots: roots, bundleID: bundleID, productIDs: productIDs, now: time.Now}, nil
}

// Verify accepts the `jwsRepresentation` of a StoreKit 2 Transaction.
// Sandbox transactions (TestFlight) are accepted like production ones.
func (v *verifier) Verify(jws string) (domain.SubscriptionTransaction, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return domain.SubscriptionTransaction{}, errors.New("malformed JWS")
	}
	var header struct {
		Alg string   `json:"alg"`
		X5c []string `json:"x5c"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return domain.SubscriptionTransaction{}, fmt.Errorf("header: %w", err)
	}
	if header.Alg != "ES256" || len(header.X5c) < 2 {
		return domain.SubscriptionTransaction{}, errors.New("unexpected JWS header")
	}
	leaf, err := v.verifyChain(header.X5c)
	if err != nil {
		return domain.SubscriptionTransaction{}, err
	}
	if err := verifyES256(leaf, parts[0]+"."+parts[1], parts[2]); err != nil {
		return domain.SubscriptionTransaction{}, err
	}

	var tx domain.SubscriptionTransaction
	if err := decodeSegment(parts[1], &tx); err != nil {
		return domain.SubscriptionTransaction{}, fmt.Errorf("payload: %w", err)
	}
	switch {
	case tx.BundleID != v.bundleID:
		return tx, fmt.Errorf("%w: bundle %q", domain.ErrNotSubscribed, tx.BundleID)
	case !slices.Contains(v.productIDs, tx.ProductID):
		return tx, fmt.Errorf("%w: product %q", domain.ErrNotSubscribed, tx.ProductID)
	case tx.RevocationDate != 0:
		return tx, fmt.Errorf("%w: revoked", domain.ErrNotSubscribed)
	case time.UnixMilli(tx.ExpiresDate).Before(v.now()):
		return tx, fmt.Errorf("%w: expired", domain.ErrNotSubscribed)
	}
	return tx, nil
}

func (v *verifier) verifyChain(x5c []string) (*x509.Certificate, error) {
	certs := make([]*x509.Certificate, len(x5c))
	for i, b64 := range x5c {
		der, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("x5c[%d]: %w", i, err)
		}
		if certs[i], err = x509.ParseCertificate(der); err != nil {
			return nil, fmt.Errorf("x5c[%d]: %w", i, err)
		}
	}
	leaf, intermediate := certs[0], certs[1]
	if !hasExtension(leaf, oidLeafMarker) || !hasExtension(intermediate, oidIntermediateMarker) {
		return nil, errors.New("certificate chain is not Apple's StoreKit chain")
	}
	intermediates := x509.NewCertPool()
	intermediates.AddCert(intermediate)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: intermediates,
		CurrentTime:   v.now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("certificate chain: %w", err)
	}
	return leaf, nil
}

func hasExtension(c *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	return slices.ContainsFunc(c.Extensions, func(e pkix.Extension) bool { return e.Id.Equal(oid) })
}

func verifyES256(cert *x509.Certificate, signingInput, signature string) error {
	key, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("leaf key is not ECDSA")
	}
	sig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || len(sig) != 64 {
		return errors.New("bad ES256 signature encoding")
	}
	digest := sha256.Sum256([]byte(signingInput))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(key, digest[:], r, s) {
		return errors.New("JWS signature does not verify")
	}
	return nil
}

func decodeSegment(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
