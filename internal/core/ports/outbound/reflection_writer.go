package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// ReflectionWriter writes the priest's reflection (Claude).
type ReflectionWriter interface {
	Write(ctx context.Context, req domain.ReflectionRequest) (string, error)
}
