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

func freeFlag(b bool) *bool { return &b }

func decideFree(s interface {
	Decide(context.Context, domain.Principal, string, domain.DecisionRequest) (json.RawMessage, error)
}, user, jws string, free *bool) error {
	req := okDecision
	req.Free = free
	_, err := s.Decide(context.Background(), domain.Principal{UserID: user}, jws, req)
	return err
}

// Only the onboarding's orientação spends the free allowance (free: true, or
// absent for app builds that predate the field). free: false without an
// active subscription is refused before anything is reserved or sent to Jev.
func TestFreeFlagWithoutSubscription(t *testing.T) {
	for _, jws := range []string{"", "expired"} {
		cases := []struct {
			name              string
			free              *bool
			wantErr           error
			reserves, fr, day int
			jev               int
		}{
			{"absent", nil, nil, 1, 1, 1, 1},
			{"true", freeFlag(true), nil, 1, 1, 1, 1},
			{"false", freeFlag(false), domain.ErrNotSubscribed, 0, 0, 0, 0},
		}
		for _, c := range cases {
			usage, jev := newMemUsage(), &decisionEngine{}
			s := NewDecisionService(decisionSubs{}, usage, jev, 40, 6)
			err := decideFree(s, "u1", jws, c.free)
			if c.wantErr == nil && err != nil || c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("jws %q, free %s: err %v, want %v", jws, c.name, err, c.wantErr)
			}
			if usage.reserves != c.reserves || usage.free["u1"] != c.fr || usage.daily["u1"] != c.day || jev.calls != c.jev {
				t.Fatalf("jws %q, free %s: reserves %d, free %d, daily %d, Jev %d; want %d, %d, %d, %d",
					jws, c.name, usage.reserves, usage.free["u1"], usage.daily["u1"], jev.calls,
					c.reserves, c.fr, c.day, c.jev)
			}
		}
	}
}

// free: false does not touch the allowance even when it is still whole: a
// later onboarding orientação gets all of it.
func TestFreeFalseLeavesTheAllowanceForTheOnboarding(t *testing.T) {
	usage, jev := newMemUsage(), &decisionEngine{}
	s := NewDecisionService(decisionSubs{}, usage, jev, 40, 2)
	for i := 0; i < 5; i++ {
		if err := decideFree(s, "u1", "", freeFlag(false)); !errors.Is(err, domain.ErrNotSubscribed) {
			t.Fatalf("free:false call %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := decideFree(s, "u1", "", freeFlag(true)); err != nil {
			t.Fatalf("onboarding call %d after free:false refusals: %v", i, err)
		}
	}
	if usage.free["u1"] != 2 || usage.daily["u1"] != 2 || jev.calls != 2 {
		t.Fatalf("free %d, daily %d, Jev %d; want 2, 2, 2", usage.free["u1"], usage.daily["u1"], jev.calls)
	}
}

// For a subscriber the flag changes nothing: the call counts against the daily
// limit only, never the free allowance (which here is zero).
func TestFreeFlagDoesNotAffectASubscriber(t *testing.T) {
	for name, free := range map[string]*bool{"absent": nil, "true": freeFlag(true), "false": freeFlag(false)} {
		usage, jev := newMemUsage(), &decisionEngine{}
		s := NewDecisionService(decisionSubs{}, usage, jev, 40, 0)
		if err := decideFree(s, "u1", "subscribed", free); err != nil {
			t.Fatalf("subscriber, free %s: %v", name, err)
		}
		if usage.free["u1"] != 0 || usage.daily["u1"] != 1 || jev.calls != 1 {
			t.Fatalf("subscriber, free %s: free %d, daily %d, Jev %d", name, usage.free["u1"], usage.daily["u1"], jev.calls)
		}
	}
}

// A forged subscription is refused before counting whatever the flag says.
func TestForgedSubscriptionIsRefusedWhateverTheFreeFlag(t *testing.T) {
	for name, free := range map[string]*bool{"absent": nil, "true": freeFlag(true), "false": freeFlag(false)} {
		usage, jev := newMemUsage(), &decisionEngine{}
		s := NewDecisionService(decisionSubs{}, usage, jev, 40, 6)
		if err := decideFree(s, "u1", "forged", free); !errors.Is(err, domain.ErrNotSubscribed) {
			t.Fatalf("forged, free %s: %v", name, err)
		}
		if usage.reserves != 0 || jev.calls != 0 {
			t.Fatalf("forged, free %s: reserves %d, Jev %d", name, usage.reserves, jev.calls)
		}
	}
}
