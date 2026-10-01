package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

// reserveUse applies the right-to-use rules shared by every call that costs
// money (Jev decisions and the padre's reflection): a verified subscription,
// or one use out of the account's free allowance when maySpendFree allows;
// then both limits are checked and counted together, before the paid call: a
// refusal by either counts nothing, but a call the provider then fails still
// spends one.
func reserveUse(ctx context.Context, subs outbound.SubscriptionVerifier, usage outbound.UsageRepository, p domain.Principal, subscriptionJWS string, maySpendFree bool, dailyLimit, freeLimit int) error {
	spendFree, err := needsFreeAllowance(subs, subscriptionJWS)
	if err != nil {
		return err
	}
	// Only the onboarding's orientação may spend the free allowance: a call
	// that opts out (free: false) without an active subscription is refused
	// before anything is counted, neither the allowance nor the daily limit.
	if spendFree && !maySpendFree {
		return domain.ErrNotSubscribed
	}
	err = usage.Reserve(ctx, outbound.Reservation{
		UserID:     p.UserID,
		DailyLimit: dailyLimit,
		SpendFree:  spendFree,
		FreeLimit:  freeLimit,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotSubscribed) || errors.Is(err, domain.ErrDailyLimit) {
			return err
		}
		return fmt.Errorf("reserve usage: %w", err)
	}
	return nil
}

// needsFreeAllowance tells whether the call has to come out of the
// account's free allowance: a subscriber's does not. Only a valid-but-inactive
// or missing subscription falls back to the allowance; a forged one is
// refused outright, before anything is counted.
func needsFreeAllowance(subs outbound.SubscriptionVerifier, subscriptionJWS string) (bool, error) {
	if subscriptionJWS == "" {
		return true, nil
	}
	_, err := subs.Verify(subscriptionJWS)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, domain.ErrNotSubscribed) {
		return false, fmt.Errorf("%w: %v", domain.ErrNotSubscribed, err)
	}
	return true, nil
}
