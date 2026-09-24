// Package subscription checks that a StoreKit 2 transaction, sent by the app
// as its signed JWS, is a live Missale subscription.
package subscription

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
)

//go:embed AppleRootCA-G3.cer
var appleRootG3 []byte

// Marker extensions Apple puts on the StoreKit signing chain.
var (
	oidLeafMarker         = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 11, 1}
	oidIntermediateMarker = asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 6, 2, 1}
)

// Transaction is the part of the JWS payload we check.
type Transaction struct {
	BundleID              string `json:"bundleId"`
	ProductID             string `json:"productId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ExpiresDate           int64  `json:"expiresDate"`
	RevocationDate        int64  `json:"revocationDate"`
	Environment           string `json:"environment"`
}

type Verifier struct {
	roots      *x509.CertPool
	bundleID   string
	productIDs []string
	now        func() time.Time
}

func NewVerifier(bundleID string, productIDs []string) (*Verifier, error) {
	root, err := x509.ParseCertificate(appleRootG3)
	if err != nil {
		return nil, fmt.Errorf("Apple root: %w", err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return &Verifier{roots: roots, bundleID: bundleID, productIDs: productIDs, now: time.Now}, nil
}

var ErrNotSubscribed = errors.New("no active subscription")

// Verify accepts the `jwsRepresentation` of a StoreKit 2 Transaction.
// Sandbox transactions (TestFlight) are accepted like production ones.
func (v *Verifier) Verify(jws string) (Transaction, error) {
	parts := strings.Split(jws, ".")
	if len(parts) != 3 {
		return Transaction{}, errors.New("malformed JWS")
	}
	var header struct {
		Alg string   `json:"alg"`
		X5c []string `json:"x5c"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return Transaction{}, fmt.Errorf("header: %w", err)
	}
	if header.Alg != "ES256" || len(header.X5c) < 2 {
		return Transaction{}, errors.New("unexpected JWS header")
	}
	leaf, err := v.verifyChain(header.X5c)
	if err != nil {
		return Transaction{}, err
	}
	if err := verifyES256(leaf, parts[0]+"."+parts[1], parts[2]); err != nil {
		return Transaction{}, err
	}

	var tx Transaction
	if err := decodeSegment(parts[1], &tx); err != nil {
		return Transaction{}, fmt.Errorf("payload: %w", err)
	}
	switch {
	case tx.BundleID != v.bundleID:
		return tx, fmt.Errorf("%w: bundle %q", ErrNotSubscribed, tx.BundleID)
	case !slices.Contains(v.productIDs, tx.ProductID):
		return tx, fmt.Errorf("%w: product %q", ErrNotSubscribed, tx.ProductID)
	case tx.RevocationDate != 0:
		return tx, fmt.Errorf("%w: revoked", ErrNotSubscribed)
	case time.UnixMilli(tx.ExpiresDate).Before(v.now()):
		return tx, fmt.Errorf("%w: expired", ErrNotSubscribed)
	}
	return tx, nil
}

func (v *Verifier) verifyChain(x5c []string) (*x509.Certificate, error) {
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
