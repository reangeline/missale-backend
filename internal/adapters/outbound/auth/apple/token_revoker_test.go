package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/golang-jwt/jwt/v5"
)

func throwawayRevoker(t *testing.T) (*tokenRevoker, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &tokenRevoker{
		keyID: "KID123", teamID: "TEAM456", clientID: "com.holymessages.app",
		key: key, http: &http.Client{Timeout: 2 * time.Second},
	}, key
}

func TestClientSecretClaimsAndSignature(t *testing.T) {
	r, key := throwawayRevoker(t)

	secret, err := r.clientSecret()
	if err != nil {
		t.Fatalf("clientSecret: %v", err)
	}

	token, err := jwt.Parse(secret, func(*jwt.Token) (any, error) { return &key.PublicKey, nil },
		jwt.WithValidMethods([]string{"ES256"}))
	if err != nil || !token.Valid {
		t.Fatalf("signature does not verify with the throwaway key: %v", err)
	}
	if token.Header["kid"] != "KID123" {
		t.Errorf("kid = %v, want KID123", token.Header["kid"])
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("claims are not a MapClaims")
	}
	if claims["iss"] != "TEAM456" {
		t.Errorf("iss = %v, want TEAM456", claims["iss"])
	}
	if claims["sub"] != "com.holymessages.app" {
		t.Errorf("sub = %v, want com.holymessages.app", claims["sub"])
	}
	if claims["aud"] != appleAudience {
		t.Errorf("aud = %v, want %s", claims["aud"], appleAudience)
	}
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	if iat == 0 || exp == 0 {
		t.Fatalf("iat/exp missing: %v", claims)
	}
	if ttl := time.Duration(exp-iat) * time.Second; ttl <= 0 || ttl > 6*30*24*time.Hour {
		t.Errorf("client secret TTL = %s, want (0, 6 months]", ttl)
	}
	if ttl := time.Duration(exp-iat) * time.Second; ttl != clientSecretTTL {
		t.Errorf("client secret TTL = %s, want %s (about 5 minutes)", ttl, clientSecretTTL)
	}
}

// exchangeAndRevokeServer fakes appleid.apple.com's /auth/token and
// /auth/revoke, checking the fields Apple documents for each.
func exchangeAndRevokeServer(t *testing.T, refreshToken string, revoked *bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("bad form: %v", err)
		}
		if form.Get("grant_type") != "authorization_code" || form.Get("code") != "the-code" ||
			form.Get("client_id") != "com.holymessages.app" || form.Get("client_secret") == "" {
			t.Errorf("unexpected /auth/token form: %v", form)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"` + refreshToken + `","token_type":"Bearer","expires_in":3600}`))
	})
	mux.HandleFunc("/auth/revoke", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("bad form: %v", err)
		}
		if form.Get("token") != refreshToken || form.Get("token_type_hint") != "refresh_token" ||
			form.Get("client_id") != "com.holymessages.app" || form.Get("client_secret") == "" {
			t.Errorf("unexpected /auth/revoke form: %v", form)
		}
		*revoked = true
		w.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux)
}

func TestRevokeExchangesAndRevokesTheToken(t *testing.T) {
	r, _ := throwawayRevoker(t)
	var revoked bool
	srv := exchangeAndRevokeServer(t, "refresh-xyz", &revoked)
	defer srv.Close()
	r.tokenURL = srv.URL + "/auth/token"
	r.revokeURL = srv.URL + "/auth/revoke"

	if err := r.Revoke(context.Background(), "the-code"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if !revoked {
		t.Error("revoke endpoint was never called")
	}
}

func TestRevokeFailsWhenExchangeFails(t *testing.T) {
	r, _ := throwawayRevoker(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	r.tokenURL = srv.URL
	r.revokeURL = srv.URL

	err := r.Revoke(context.Background(), "the-code")
	if err == nil {
		t.Fatal("expected an error when Apple refuses the code")
	}
}

func TestRevokeFailsWhenRevokeCallFails(t *testing.T) {
	r, _ := throwawayRevoker(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"refresh_token":"rt"}`))
	})
	mux.HandleFunc("/auth/revoke", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	r.tokenURL = srv.URL + "/auth/token"
	r.revokeURL = srv.URL + "/auth/revoke"

	if err := r.Revoke(context.Background(), "the-code"); err == nil {
		t.Fatal("expected an error when Apple's revoke call fails")
	}
}

func TestRevokeWithoutAKeyIsDisabled(t *testing.T) {
	r := &tokenRevoker{http: &http.Client{}} // key is nil: not configured
	if err := r.Revoke(context.Background(), "the-code"); err == nil {
		t.Fatal("expected revocation to be refused without a key")
	}
}

func TestNewTokenRevokerWithoutASecretNameSkipsRevocation(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	revoker := NewTokenRevoker(context.Background(), aws.Config{}, "", "kid", "team", "com.holymessages.app", log)
	if err := revoker.Revoke(context.Background(), "the-code"); err == nil {
		t.Fatal("expected revocation to be disabled when no secret name is configured")
	}
}
