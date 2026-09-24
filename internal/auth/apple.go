package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
	appleIssuer  = "https://appleid.apple.com"
)

// AppleIdentity is what we keep from a Sign in with Apple identity token.
// Email comes only on the first sign-in and may be a private relay address.
type AppleIdentity struct {
	Sub   string
	Email string
}

type AppleVerifier struct {
	jwks     *JWKS
	audience string
}

func NewAppleVerifier(bundleID string) *AppleVerifier {
	return &AppleVerifier{jwks: NewJWKS(appleJWKSURL), audience: bundleID}
}

// Verify checks the identity token the app got from ASAuthorizationAppleIDCredential.
func (v *AppleVerifier) Verify(ctx context.Context, identityToken string) (AppleIdentity, error) {
	token, err := jwt.Parse(identityToken, v.jwks.Keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return AppleIdentity{}, fmt.Errorf("invalid Apple token: %w", err)
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return AppleIdentity{}, errors.New("Apple token without sub")
	}
	email, _ := claims["email"].(string)
	return AppleIdentity{Sub: sub, Email: email}, nil
}
