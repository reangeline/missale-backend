package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type UserRepository interface {
	// Save records the account; saving an existing one is a no-op.
	Save(ctx context.Context, u domain.User) error
	// Delete removes the account and its usage rows.
	Delete(ctx context.Context, userID string) error
}
