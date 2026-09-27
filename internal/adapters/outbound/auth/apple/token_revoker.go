package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt/v5"

	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

const (
	appleTokenURL  = "https://appleid.apple.com/auth/token"
	appleRevokeURL = "https://appleid.apple.com/auth/revoke"
	appleAudience  = "https://appleid.apple.com"

	// clientSecretTTL is well under Apple's six-month ceiling for this JWT;
	// each one signs a single request, so there's no reason to let it outlive it.
	clientSecretTTL = 5 * time.Minute
)

type tokenRevoker struct {
	keyID, teamID, clientID string
	key                     *ecdsa.PrivateKey // nil disables revocation
	http                    *http.Client
	tokenURL, revokeURL     string
}

// NewTokenRevoker reads the Sign in with Apple private key from Secrets
// Manager once, at cold start (secretName, e.g. "missale/dev/apple-signin-key").
// A missing secret, or one that isn't a usable key, does not stop the API
// from starting: revocation is skipped, with a log warning, and account
// deletion keeps working without it.
func NewTokenRevoker(ctx context.Context, awsCfg aws.Config, secretName, keyID, teamID, clientID string, log *slog.Logger) outbound.AppleTokenRevoker {
	r := &tokenRevoker{
		keyID: keyID, teamID: teamID, clientID: clientID,
		http:      &http.Client{Timeout: 10 * time.Second},
		tokenURL:  appleTokenURL,
		revokeURL: appleRevokeURL,
	}
	if secretName == "" {
		log.Warn("no Apple Sign in key secret configured; account deletion will not revoke Apple tokens")
		return r
	}
	pemStr, err := fetchSecret(ctx, awsCfg, secretName)
	if err != nil {
		log.Warn("could not read the Apple Sign in key; account deletion will not revoke Apple tokens",
			"secret", secretName, "err", err)
		return r
	}
	key, err := parseECPrivateKey(pemStr)
	if err != nil {
		log.Warn("the Apple Sign in key secret is not a usable EC private key; account deletion will not revoke Apple tokens",
			"secret", secretName, "err", err)
		return r
	}
	r.key = key
	return r
}

func fetchSecret(ctx context.Context, awsCfg aws.Config, name string) (string, error) {
	out, err := secretsmanager.NewFromConfig(awsCfg).GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	})
	if err != nil {
		return "", err
	}
	if out.SecretString == nil || *out.SecretString == "" {
		return "", errors.New("secret has no string value")
	}
	return *out.SecretString, nil
}

// parseECPrivateKey reads the .p8 PEM Apple gives for Sign in with Apple keys.
func parseECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("not a PEM block")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	generic, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := generic.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an EC private key")
	}
	return key, nil
}

// clientSecret builds the ES256 JWT Apple calls the "client secret": proof
// that we hold the Sign in with Apple private key for this team and app.
func (r *tokenRevoker) clientSecret() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": r.teamID,
		"iat": now.Unix(),
		"exp": now.Add(clientSecretTTL).Unix(),
		"aud": appleAudience,
		"sub": r.clientID,
	})
	token.Header["kid"] = r.keyID
	return token.SignedString(r.key)
}

// Revoke exchanges the authorization code for Apple's tokens and revokes the
// refresh token. A failure here must never block account deletion: the
// account service logs it (this error carries no secret) and moves on.
func (r *tokenRevoker) Revoke(ctx context.Context, authorizationCode string) error {
	if r.key == nil {
		return errors.New("apple token revocation is not configured")
	}
	if authorizationCode == "" {
		return errors.New("empty authorization code")
	}
	secret, err := r.clientSecret()
	if err != nil {
		return fmt.Errorf("client secret: %w", err)
	}
	refreshToken, err := r.exchangeCode(ctx, authorizationCode, secret)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}
	if err := r.revokeToken(ctx, refreshToken, secret); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

func (r *tokenRevoker) exchangeCode(ctx context.Context, code, clientSecret string) (string, error) {
	form := url.Values{
		"client_id":     {r.clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	}
	var out struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := r.post(ctx, r.tokenURL, form, &out); err != nil {
		return "", err
	}
	if out.RefreshToken == "" {
		return "", errors.New("Apple did not return a refresh token")
	}
	return out.RefreshToken, nil
}

func (r *tokenRevoker) revokeToken(ctx context.Context, refreshToken, clientSecret string) error {
	form := url.Values{
		"client_id":       {r.clientID},
		"client_secret":   {clientSecret},
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
	}
	return r.post(ctx, r.revokeURL, form, nil)
}

// post sends one application/x-www-form-urlencoded request and decodes a
// JSON response into out (nil when the caller doesn't need the body). Errors
// never include the form (which may hold the code, client secret or token).
func (r *tokenRevoker) post(ctx context.Context, target string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return errors.New("could not build the request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := r.http.Do(req)
	if err != nil {
		return errors.New("request to Apple failed")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Apple answered HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errors.New("unexpected response from Apple")
	}
	return nil
}
