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
	// freeDecisions is each account's lifetime allowance without a
	// subscription: the orientação the onboarding offers (two calls: state
	// and risk, then the reviewed reply).
	freeDecisions int
}

func NewDecisionService(subscriptions outbound.SubscriptionVerifier, usage outbound.UsageRepository, engine outbound.DecisionEngine, dailyLimit, freeDecisions int) inbound.DecisionService {
	return &decisionService{subscriptions: subscriptions, usage: usage, engine: engine, dailyLimit: dailyLimit, freeDecisions: freeDecisions}
}

func (s *decisionService) Decide(ctx context.Context, p domain.Principal, subscriptionJWS string, req domain.DecisionRequest) (json.RawMessage, error) {
	if err := Validate(req); err != nil {
		return nil, err
	}
	spendFree, err := s.needsFreeAllowance(subscriptionJWS)
	if err != nil {
		return nil, err
	}
	// Only the onboarding's orientação may spend the free allowance: a call
	// that opts out (free: false) without an active subscription is refused
	// before anything is counted, neither the allowance nor the daily limit.
	if spendFree && !req.MaySpendFree() {
		return nil, domain.ErrNotSubscribed
	}
	// Both limits are checked and counted together, before calling Jev: a
	// refusal by either counts nothing, but a call Jev then fails still
	// spends one (simpler, and it has not happened in production).
	err = s.usage.Reserve(ctx, outbound.Reservation{
		UserID:     p.UserID,
		DailyLimit: s.dailyLimit,
		SpendFree:  spendFree,
		FreeLimit:  s.freeDecisions,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotSubscribed) || errors.Is(err, domain.ErrDailyLimit) {
			return nil, err
		}
		return nil, fmt.Errorf("reserve usage: %w", err)
	}
	answers, err := s.engine.Decide(ctx, req.State, req.Questions)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrDecisionEngine, err)
	}
	return answers, nil
}

// needsFreeAllowance tells whether the call has to come out of the
// account's free allowance: a subscriber's does not. Only a valid-but-inactive
// or missing subscription falls back to the allowance; a forged one is
// refused outright, before anything is counted.
func (s *decisionService) needsFreeAllowance(subscriptionJWS string) (bool, error) {
	if subscriptionJWS == "" {
		return true, nil
	}
	_, err := s.subscriptions.Verify(subscriptionJWS)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, domain.ErrNotSubscribed) {
		return false, fmt.Errorf("%w: %v", domain.ErrNotSubscribed, err)
	}
	return true, nil
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
