package outbound

import "context"

type UsageRepository interface {
	// Reserve counts one decision for today (UTC) and returns today's total.
	Reserve(ctx context.Context, userID string) (int, error)
}
