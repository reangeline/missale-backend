package inbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// AdminAuthService signs in to the admin page with email and password.
type AdminAuthService interface {
	SignIn(ctx context.Context, email, password string) (domain.AdminSession, error)
	// CompleteNewPassword answers the first sign-in's challenge (Cognito's
	// temporary password) with the password the admin chose.
	CompleteNewPassword(ctx context.Context, email, session, newPassword string) (domain.AdminSession, error)
	Refresh(ctx context.Context, refreshToken string) (domain.AdminSession, error)
	// Authenticate resolves an access token to an admin; anyone outside the
	// admin group gets domain.ErrForbidden.
	Authenticate(ctx context.Context, accessToken string) (domain.Admin, error)
}
