package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// IdentityVerifier checks a Sign in with Apple identity token.
type IdentityVerifier interface {
	Verify(ctx context.Context, identityToken string) (domain.AppleIdentity, error)
}
