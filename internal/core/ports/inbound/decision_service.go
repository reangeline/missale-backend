package inbound

import (
	"context"
	"encoding/json"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type DecisionService interface {
	// Decide checks the subscription and the daily limit, then asks Jev.
	// subscriptionJWS is the StoreKit 2 transaction's jwsRepresentation.
	Decide(ctx context.Context, p domain.Principal, subscriptionJWS string, req domain.DecisionRequest) (json.RawMessage, error)
}
