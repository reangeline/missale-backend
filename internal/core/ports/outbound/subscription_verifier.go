package outbound

import "github.com/reangeline/missale-backend/internal/core/domain"

// SubscriptionVerifier checks a StoreKit 2 transaction JWS.
// It returns domain.ErrNotSubscribed for valid but inactive transactions.
type SubscriptionVerifier interface {
	Verify(jws string) (domain.SubscriptionTransaction, error)
}
