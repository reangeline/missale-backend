// Package apple verifies Sign in with Apple identity tokens.
package apple

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/reangeline/missale-backend/internal/adapters/outbound/auth/jwks"
	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

const (
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
	appleIssuer  = "https://appleid.apple.com"
)

type identityVerifier struct {
	jwks     *jwks.JWKS
	audience string
}

func NewIdentityVerifier(bundleID string) outbound.IdentityVerifier {
	return &identityVerifier{jwks: jwks.New(appleJWKSURL), audience: bundleID}
}

// Verify checks the identity token the app got from ASAuthorizationAppleIDCredential.
func (v *identityVerifier) Verify(ctx context.Context, identityToken string) (domain.AppleIdentity, error) {
	token, err := jwt.Parse(identityToken, v.jwks.Keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return domain.AppleIdentity{}, fmt.Errorf("invalid Apple token: %w", err)
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return domain.AppleIdentity{}, errors.New("Apple token without sub")
	}
	email, _ := claims["email"].(string)
	return domain.AppleIdentity{Sub: sub, Email: email}, nil
}
