package inbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type AuthService interface {
	// SignInWithApple verifies the Apple identity token and returns a session,
	// creating the account on the first sign-in.
	SignInWithApple(ctx context.Context, identityToken string) (domain.Session, error)
	RefreshSession(ctx context.Context, refreshToken string) (domain.Session, error)
	// Authenticate resolves an access token to its owner.
	Authenticate(ctx context.Context, accessToken string) (domain.Principal, error)
}
