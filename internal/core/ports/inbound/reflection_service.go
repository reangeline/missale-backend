package inbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type ReflectionService interface {
	// Reflect applies the same right-to-use rules as Decide (subscription or
	// free allowance, daily limit; one use), then asks Claude for the reflection.
	Reflect(ctx context.Context, p domain.Principal, subscriptionJWS string, req domain.ReflectionRequest) (string, error)
}
