package outbound

import "context"

type UsageRepository interface {
	// Reserve counts one decision for today (UTC) and returns today's total.
	Reserve(ctx context.Context, userID string) (int, error)
	// ReserveFree counts one decision against the account's lifetime free
	// allowance (the orientação offered in the onboarding) and returns the total.
	ReserveFree(ctx context.Context, userID string) (int, error)
}
