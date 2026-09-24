package outbound

import (
	"context"
	"encoding/json"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// DecisionEngine answers typed questions about a text (Jev).
type DecisionEngine interface {
	Decide(ctx context.Context, state string, questions map[string]domain.Question) (json.RawMessage, error)
}
