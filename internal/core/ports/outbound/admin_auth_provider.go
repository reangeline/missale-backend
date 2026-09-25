package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// AdminAuthProvider is the admin page's sign-in (Cognito, email + password).
type AdminAuthProvider interface {
	SignIn(ctx context.Context, email, password string) (domain.AdminSession, error)
	RespondNewPassword(ctx context.Context, email, session, newPassword string) (domain.AdminSession, error)
	Refresh(ctx context.Context, refreshToken string) (domain.AdminSession, error)
	// Verify returns the admin behind a token and the groups they belong to.
	Verify(ctx context.Context, accessToken string) (domain.Admin, []string, error)
}
