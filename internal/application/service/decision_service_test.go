package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

// memUsage keeps the repository's contract per account: a limit already
// reached is refused and nothing is counted, all or nothing.
type memUsage struct {
	free, daily map[string]int
	reserves    int
}

func newMemUsage() *memUsage {
	return &memUsage{free: map[string]int{}, daily: map[string]int{}}
}

func (m *memUsage) Reserve(_ context.Context, r outbound.Reservation) error {
	m.reserves++
	if r.SpendFree && m.free[r.UserID] >= r.FreeLimit {
		return domain.ErrNotSubscribed
	}
	if m.daily[r.UserID] >= r.DailyLimit {
		return domain.ErrDailyLimit
	}
	if r.SpendFree {
		m.free[r.UserID]++
	}
	m.daily[r.UserID]++
	return nil
}

type decisionSubs struct{}

func (decisionSubs) Verify(jws string) (domain.SubscriptionTransaction, error) {
	switch jws {
	case "subscribed":
		return domain.SubscriptionTransaction{ProductID: "anual"}, nil
	case "expired":
		return domain.SubscriptionTransaction{}, domain.ErrNotSubscribed
	}
	return domain.SubscriptionTransaction{}, errors.New("signature does not verify")
}

type decisionEngine struct {
	calls int
	err   error
}

func (e *decisionEngine) Decide(context.Context, string, map[string]domain.Question) (json.RawMessage, error) {
	e.calls++
	if e.err != nil {
		return nil, e.err
	}
	return json.RawMessage(`{"risk":{"type":"noul","noul":0.01}}`), nil
}

var okDecision = domain.DecisionRequest{
	State:     "Hoje eu estou cansado.",
	Questions: map[string]domain.Question{"risk": {Type: "noul", Instructions: "Risk?"}},
}

func decide(s interface {
	Decide(context.Context, domain.Principal, string, domain.DecisionRequest) (json.RawMessage, error)
}, user, jws string) error {
	_, err := s.Decide(context.Background(), domain.Principal{UserID: user}, jws, okDecision)
	return err
}

// The free allowance serves exactly its size; every call after it is refused,
// and refused calls do not count (the old counter kept climbing past the limit,
// so raising FREE_DECISIONS never helped an account that had been refused).
func TestFreeAllowanceRefusalsDoNotSpend(t *testing.T) {
	usage, jev := newMemUsage(), &decisionEngine{}
	s := NewDecisionService(decisionSubs{}, usage, jev, 40, 6)

	for i := 1; i <= 6; i++ {
		if err := decide(s, "u1", ""); err != nil {
			t.Fatalf("free call %d: %v", i, err)
		}
	}
	for i := 0; i < 11; i++ {
		if err := decide(s, "u1", ""); !errors.Is(err, domain.ErrNotSubscribed) {
			t.Fatalf("refusal %d: %v", i, err)
		}
	}
	if usage.free["u1"] != 6 || usage.daily["u1"] != 6 || jev.calls != 6 {
		t.Fatalf("after 11 refusals: free %d, daily %d, Jev %d; want 6, 6, 6",
			usage.free["u1"], usage.daily["u1"], jev.calls)
	}

	// Raising the limit now gives the account exactly the extra calls.
	s = NewDecisionService(decisionSubs{}, usage, jev, 40, 8)
	for i := 0; i < 2; i++ {
		if err := decide(s, "u1", "expired"); err != nil {
			t.Fatalf("call %d after raising the limit: %v", i, err)
		}
	}
	if err := decide(s, "u1", ""); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("past the raised limit: %v", err)
	}
}

func TestFreeAllowanceIsPerAccount(t *testing.T) {
	usage := newMemUsage()
	s := NewDecisionService(decisionSubs{}, usage, &decisionEngine{}, 40, 2)

	for i := 0; i < 2; i++ {
		if err := decide(s, "u1", ""); err != nil {
			t.Fatalf("u1 call %d: %v", i, err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := decide(s, "u1", ""); !errors.Is(err, domain.ErrNotSubscribed) {
			t.Fatalf("u1 refusal %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := decide(s, "u2", ""); err != nil {
			t.Fatalf("u2 call %d after u1 ran out: %v", i, err)
		}
	}
	if err := decide(s, "u2", ""); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("u2 past the limit: %v", err)
	}
	if usage.free["u1"] != 2 || usage.free["u2"] != 2 {
		t.Fatalf("free: u1 %d, u2 %d", usage.free["u1"], usage.free["u2"])
	}
}

// A refusal by the daily limit does not spend the lifetime free allowance.
func TestDailyRefusalDoesNotSpendTheFreeAllowance(t *testing.T) {
	usage, jev := newMemUsage(), &decisionEngine{}
	s := NewDecisionService(decisionSubs{}, usage, jev, 3, 6)

	for i := 0; i < 3; i++ { // a subscriber uses today's calls, then lapses
		if err := decide(s, "u1", "subscribed"); err != nil {
			t.Fatalf("subscribed call %d: %v", i, err)
		}
	}
	for i := 0; i < 4; i++ {
		if err := decide(s, "u1", "expired"); !errors.Is(err, domain.ErrDailyLimit) {
			t.Fatalf("daily refusal %d: %v", i, err)
		}
	}
	if usage.free["u1"] != 0 || usage.daily["u1"] != 3 || jev.calls != 3 {
		t.Fatalf("free %d, daily %d, Jev %d; want 0, 3, 3", usage.free["u1"], usage.daily["u1"], jev.calls)
	}
}

func TestSubscriberDoesNotSpendTheFreeAllowance(t *testing.T) {
	usage := newMemUsage()
	s := NewDecisionService(decisionSubs{}, usage, &decisionEngine{}, 40, 0)
	if err := decide(s, "u1", "subscribed"); err != nil {
		t.Fatalf("subscriber with no free allowance: %v", err)
	}
	if usage.free["u1"] != 0 || usage.daily["u1"] != 1 {
		t.Fatalf("free %d, daily %d", usage.free["u1"], usage.daily["u1"])
	}
}

func TestForgedSubscriptionIsRefusedBeforeCounting(t *testing.T) {
	usage, jev := newMemUsage(), &decisionEngine{}
	s := NewDecisionService(decisionSubs{}, usage, jev, 40, 6)
	if err := decide(s, "u1", "forged"); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("forged: %v", err)
	}
	if usage.reserves != 0 || jev.calls != 0 {
		t.Fatalf("forged reached usage (%d) or Jev (%d)", usage.reserves, jev.calls)
	}
}

// Current policy, kept on purpose: the call is counted before Jev, so a Jev
// failure still spends it.
func TestEngineFailureStillSpendsTheReservedCall(t *testing.T) {
	usage := newMemUsage()
	s := NewDecisionService(decisionSubs{}, usage, &decisionEngine{err: errors.New("timeout")}, 40, 6)
	if err := decide(s, "u1", ""); !errors.Is(err, domain.ErrDecisionEngine) {
		t.Fatalf("engine failure: %v", err)
	}
	if usage.free["u1"] != 1 || usage.daily["u1"] != 1 {
		t.Fatalf("free %d, daily %d; want 1, 1", usage.free["u1"], usage.daily["u1"])
	}
}

func TestStoreFailureIsNotARefusal(t *testing.T) {
	s := NewDecisionService(decisionSubs{}, failingUsage{}, &decisionEngine{}, 40, 6)
	err := decide(s, "u1", "")
	if err == nil || errors.Is(err, domain.ErrNotSubscribed) || errors.Is(err, domain.ErrDailyLimit) {
		t.Fatalf("store failure: %v", err)
	}
}

type failingUsage struct{}

func (failingUsage) Reserve(context.Context, outbound.Reservation) error {
	return errors.New("connection reset")
}
