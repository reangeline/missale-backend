package inbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type AccountService interface {
	// DeleteAccount erases everything the server holds about the user.
	// authorizationCode, when the app sends one, is a fresh Sign in with
	// Apple code used once to revoke the Apple token (guideline 5.1.1(v));
	// empty for app versions that don't send it, which keeps today's behavior.
	DeleteAccount(ctx context.Context, p domain.Principal, authorizationCode string) error
}
