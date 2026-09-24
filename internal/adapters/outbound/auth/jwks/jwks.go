// Package jwks caches the RSA signing keys published by Apple and Cognito.
package jwks

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWKS caches the RSA keys published at one URL (Apple, Cognito) and refetches
// once when a token names a key it doesn't know, to follow key rotation.
// Adapted from Hirefy's social token validator.
type JWKS struct {
	url     string
	client  *http.Client
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

const jwksTTL = time.Hour

func New(url string) *JWKS {
	return &JWKS{url: url, client: &http.Client{Timeout: 10 * time.Second}}
}

// Keyfunc resolves the RS256 key for a token by its kid.
func (j *JWKS) Keyfunc(ctx context.Context) jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if key := j.lookup(kid); key != nil {
			return key, nil
		}
		if err := j.refresh(ctx); err != nil {
			return nil, err
		}
		if key := j.lookup(kid); key != nil {
			return key, nil
		}
		return nil, fmt.Errorf("signing key %q not found", kid)
	}
}

func (j *JWKS) lookup(kid string) *rsa.PublicKey {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if time.Since(j.fetched) > jwksTTL {
		return nil
	}
	return j.keys[kid]
}

func (j *JWKS) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS %s returned HTTP %d", j.url, resp.StatusCode)
	}
	var raw struct {
		Keys []struct{ Kid, Kty, N, E string } `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return fmt.Errorf("decode JWKS: %w", err)
	}
	keys := make(map[string]*rsa.PublicKey, len(raw.Keys))
	for _, k := range raw.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("bad key %q in JWKS", k.Kid)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	j.mu.Lock()
	j.keys, j.fetched = keys, time.Now()
	j.mu.Unlock()
	return nil
}
