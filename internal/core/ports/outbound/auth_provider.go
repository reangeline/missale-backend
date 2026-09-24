package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// AuthProvider issues and checks sessions (Cognito).
type AuthProvider interface {
	// SignIn returns the account's stable ID and a new session, creating the
	// account if it doesn't exist yet.
	SignIn(ctx context.Context, identity domain.AppleIdentity) (string, domain.Session, error)
	Refresh(ctx context.Context, refreshToken string) (domain.Session, error)
	Verify(ctx context.Context, accessToken string) (domain.Principal, error)
	Delete(ctx context.Context, username string) error
}
