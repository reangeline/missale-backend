package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type decisionService struct {
	subscriptions outbound.SubscriptionVerifier
	usage         outbound.UsageRepository
	engine        outbound.DecisionEngine
	dailyLimit    int
}

func NewDecisionService(subscriptions outbound.SubscriptionVerifier, usage outbound.UsageRepository, engine outbound.DecisionEngine, dailyLimit int) inbound.DecisionService {
	return &decisionService{subscriptions: subscriptions, usage: usage, engine: engine, dailyLimit: dailyLimit}
}

func (s *decisionService) Decide(ctx context.Context, p domain.Principal, subscriptionJWS string, req domain.DecisionRequest) (json.RawMessage, error) {
	if _, err := s.subscriptions.Verify(subscriptionJWS); err != nil {
		if errors.Is(err, domain.ErrNotSubscribed) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrNotSubscribed, err)
	}
	if err := Validate(req); err != nil {
		return nil, err
	}
	// Counted before calling Jev, so a failed call still spends one: simpler,
	// and the daily limit is generous.
	calls, err := s.usage.Reserve(ctx, p.UserID)
	if err != nil {
		return nil, fmt.Errorf("reserve usage: %w", err)
	}
	if calls > s.dailyLimit {
		return nil, domain.ErrDailyLimit
	}
	answers, err := s.engine.Decide(ctx, req.State, req.Questions)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrDecisionEngine, err)
	}
	return answers, nil
}

// Validate enforces the domain limits on a decision request.
func Validate(req domain.DecisionRequest) error {
	if strings.TrimSpace(req.State) == "" || utf8.RuneCountInString(req.State) > domain.MaxStateChars {
		return domain.ErrInvalidState
	}
	if len(req.Questions) == 0 || len(req.Questions) > domain.MaxQuestions {
		return domain.ErrInvalidQuestions
	}
	for key, q := range req.Questions {
		if key == "" || q.Instructions == "" || utf8.RuneCountInString(q.Instructions) > domain.MaxInstructionsChars {
			return domain.ErrInvalidQuestions
		}
		var texts []string
		switch q.Type {
		case "noul":
			if len(q.Criteria) != 0 {
				return domain.ErrInvalidQuestions
			}
			continue
		case "choice":
			var criteria map[string]string
			if json.Unmarshal(q.Criteria, &criteria) != nil {
				return domain.ErrInvalidQuestions
			}
			for k, v := range criteria {
				if k == "" {
					return domain.ErrInvalidQuestions
				}
				texts = append(texts, v)
			}
		case "score":
			if json.Unmarshal(q.Criteria, &texts) != nil {
				return domain.ErrInvalidQuestions
			}
		default:
			return domain.ErrInvalidQuestions
		}
		if len(texts) < 2 || len(texts) > domain.MaxCriteria {
			return domain.ErrInvalidQuestions
		}
		for _, t := range texts {
			if t == "" || utf8.RuneCountInString(t) > domain.MaxCriterionChars {
				return domain.ErrInvalidQuestions
			}
		}
	}
	return nil
}
