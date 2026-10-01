package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type reflectionService struct {
	subscriptions outbound.SubscriptionVerifier
	usage         outbound.UsageRepository
	writer        outbound.ReflectionWriter // nil: no Anthropic key configured
	dailyLimit    int
	freeDecisions int
}

// NewReflectionService: a nil writer makes every call fail with
// ErrReflectionNotEnabled, before anything is counted.
func NewReflectionService(subscriptions outbound.SubscriptionVerifier, usage outbound.UsageRepository, writer outbound.ReflectionWriter, dailyLimit, freeDecisions int) inbound.ReflectionService {
	return &reflectionService{subscriptions: subscriptions, usage: usage, writer: writer, dailyLimit: dailyLimit, freeDecisions: freeDecisions}
}

func (s *reflectionService) Reflect(ctx context.Context, p domain.Principal, subscriptionJWS string, req domain.ReflectionRequest) (string, error) {
	if s.writer == nil {
		return "", domain.ErrReflectionNotEnabled
	}
	if err := ValidateReflection(req); err != nil {
		return "", err
	}
	if err := reserveUse(ctx, s.subscriptions, s.usage, p, subscriptionJWS, req.MaySpendFree(), s.dailyLimit, s.freeDecisions); err != nil {
		return "", err
	}
	text, err := s.writer.Write(ctx, req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrReflectionEngine, err)
	}
	return text, nil
}

// ValidateReflection enforces the domain limits on a reflection request.
func ValidateReflection(req domain.ReflectionRequest) error {
	if strings.TrimSpace(req.State) == "" || utf8.RuneCountInString(req.State) > domain.MaxStateChars {
		return domain.ErrInvalidState
	}
	if !domain.ReflectionLanguages[req.Language] {
		return domain.ErrInvalidReflection
	}
	for _, f := range []struct {
		s   string
		max int
	}{
		{req.Passage.Reference, domain.MaxLabelChars},
		{req.Saint.Name, domain.MaxLabelChars},
	} {
		if strings.TrimSpace(f.s) == "" || utf8.RuneCountInString(f.s) > f.max {
			return domain.ErrInvalidReflection
		}
	}
	if utf8.RuneCountInString(req.Passage.Text) > domain.MaxPassageChars || utf8.RuneCountInString(req.Saint.Summary) > domain.MaxSummaryChars {
		return domain.ErrInvalidReflection
	}
	return nil
}
