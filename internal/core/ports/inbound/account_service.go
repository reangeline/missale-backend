package inbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type AccountService interface {
	// DeleteAccount erases everything the server holds about the user.
	DeleteAccount(ctx context.Context, p domain.Principal) error
}
