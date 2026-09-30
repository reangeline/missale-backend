package outbound

import "context"

// Reservation is what one decision spends: always a call from today's (UTC)
// limit and, for an account without a subscription, one from its lifetime
// free allowance (the orientação offered in the onboarding).
type Reservation struct {
	UserID     string
	DailyLimit int
	SpendFree  bool
	FreeLimit  int
}

type UsageRepository interface {
	// Reserve counts the decision against every limit it spends, all or
	// nothing, in one transaction. A limit already reached is refused with
	// domain.ErrNotSubscribed (free allowance, checked first) or
	// domain.ErrDailyLimit, and then nothing is counted at all.
	Reserve(ctx context.Context, r Reservation) error
}
