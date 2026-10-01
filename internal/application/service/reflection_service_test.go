package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type fakeWriter struct {
	calls int
	err   error
}

func (w *fakeWriter) Write(context.Context, domain.ReflectionRequest) (string, error) {
	w.calls++
	if w.err != nil {
		return "", w.err
	}
	return "reflexão", nil
}

var okReflection = domain.ReflectionRequest{
	State:    "Estou cansado.",
	Passage:  domain.Passage{Reference: "Mt 11,28", Text: "Vinde a mim."},
	Saint:    domain.Saint{Name: "Santa Mônica", Summary: "Rezou."},
	Language: "pt",
}

func reflect(s interface {
	Reflect(context.Context, domain.Principal, string, domain.ReflectionRequest) (string, error)
}, user, jws string, req domain.ReflectionRequest) error {
	_, err := s.Reflect(context.Background(), domain.Principal{UserID: user}, jws, req)
	return err
}

func TestReflectSubscriberDoesNotSpendFree(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{}
	s := NewReflectionService(decisionSubs{}, usage, w, 5, 3)
	if err := reflect(s, "u", "subscribed", okReflection); err != nil {
		t.Fatal(err)
	}
	if usage.free["u"] != 0 || usage.daily["u"] != 1 || w.calls != 1 {
		t.Fatalf("free %d daily %d writer %d", usage.free["u"], usage.daily["u"], w.calls)
	}
}

func TestReflectFreeAllowanceThenRefused(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{}
	s := NewReflectionService(decisionSubs{}, usage, w, 40, 3)
	for i := 0; i < 3; i++ {
		if err := reflect(s, "u", "", okReflection); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if err := reflect(s, "u", "expired", okReflection); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("4th: %v", err)
	}
	if w.calls != 3 || usage.free["u"] != 3 {
		t.Fatalf("writer %d free %d", w.calls, usage.free["u"])
	}
}

func TestReflectFreeFalseWithoutSubscriptionSpendsNothing(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{}
	s := NewReflectionService(decisionSubs{}, usage, w, 5, 3)
	no := false
	req := okReflection
	req.Free = &no
	if err := reflect(s, "u", "", req); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("got %v", err)
	}
	if usage.reserves != 0 || w.calls != 0 {
		t.Fatalf("reserves %d writer %d", usage.reserves, w.calls)
	}
}

func TestReflectForgedSubscriptionRefused(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{}
	s := NewReflectionService(decisionSubs{}, usage, w, 5, 3)
	if err := reflect(s, "u", "forged", okReflection); !errors.Is(err, domain.ErrNotSubscribed) {
		t.Fatalf("got %v", err)
	}
	if usage.reserves != 0 || w.calls != 0 {
		t.Fatalf("reserves %d writer %d", usage.reserves, w.calls)
	}
}

func TestReflectDailyLimit(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{}
	s := NewReflectionService(decisionSubs{}, usage, w, 2, 3)
	for i := 0; i < 2; i++ {
		if err := reflect(s, "u", "subscribed", okReflection); err != nil {
			t.Fatal(err)
		}
	}
	if err := reflect(s, "u", "subscribed", okReflection); !errors.Is(err, domain.ErrDailyLimit) {
		t.Fatalf("got %v", err)
	}
	if w.calls != 2 {
		t.Fatalf("writer %d", w.calls)
	}
}

func TestReflectWriterFailureIsReflectionEngineError(t *testing.T) {
	usage, w := newMemUsage(), &fakeWriter{err: errors.New("claude: refusal (bio)")}
	s := NewReflectionService(decisionSubs{}, usage, w, 5, 3)
	err := reflect(s, "u", "subscribed", okReflection)
	if !errors.Is(err, domain.ErrReflectionEngine) {
		t.Fatalf("got %v", err)
	}
	if usage.daily["u"] != 1 { // like Jev: a call the provider fails still spends one
		t.Fatalf("daily %d", usage.daily["u"])
	}
}

func TestReflectWithoutWriterIsNotEnabledAndSpendsNothing(t *testing.T) {
	usage := newMemUsage()
	s := NewReflectionService(decisionSubs{}, usage, nil, 5, 3)
	if err := reflect(s, "u", "subscribed", okReflection); !errors.Is(err, domain.ErrReflectionNotEnabled) {
		t.Fatalf("got %v", err)
	}
	if usage.reserves != 0 {
		t.Fatal("spent")
	}
}

func TestValidateReflection(t *testing.T) {
	mut := func(f func(*domain.ReflectionRequest)) domain.ReflectionRequest {
		r := okReflection
		f(&r)
		return r
	}
	cases := map[string]struct {
		req  domain.ReflectionRequest
		want error
	}{
		"ok":               {okReflection, nil},
		"empty summary ok": {mut(func(r *domain.ReflectionRequest) { r.Saint.Summary = "" }), nil},
		"empty state":      {mut(func(r *domain.ReflectionRequest) { r.State = "  " }), domain.ErrInvalidState},
		"state too long":   {mut(func(r *domain.ReflectionRequest) { r.State = strings.Repeat("a", domain.MaxStateChars+1) }), domain.ErrInvalidState},
		"no reference":     {mut(func(r *domain.ReflectionRequest) { r.Passage.Reference = "" }), domain.ErrInvalidReflection},
		"no name":          {mut(func(r *domain.ReflectionRequest) { r.Saint.Name = "" }), domain.ErrInvalidReflection},
		"passage too long": {mut(func(r *domain.ReflectionRequest) { r.Passage.Text = strings.Repeat("a", domain.MaxPassageChars+1) }), domain.ErrInvalidReflection},
		"summary too long": {mut(func(r *domain.ReflectionRequest) { r.Saint.Summary = strings.Repeat("a", domain.MaxSummaryChars+1) }), domain.ErrInvalidReflection},
		"context at limit": {mut(func(r *domain.ReflectionRequest) { r.Context = strings.Repeat("é", domain.MaxContextChars) }), nil},
		"context too long": {mut(func(r *domain.ReflectionRequest) { r.Context = strings.Repeat("é", domain.MaxContextChars+1) }), domain.ErrInvalidReflection},
		"crisis without passage and saint": {mut(func(r *domain.ReflectionRequest) {
			r.Crisis, r.Passage, r.Saint = true, domain.Passage{}, domain.Saint{}
		}), nil},
		"crisis keeps the limits": {mut(func(r *domain.ReflectionRequest) {
			r.Crisis = true
			r.Passage.Reference = strings.Repeat("a", domain.MaxLabelChars+1)
		}), domain.ErrInvalidReflection},
		"crisis still needs state": {mut(func(r *domain.ReflectionRequest) { r.Crisis = true; r.State = "" }), domain.ErrInvalidState},
		"language":                 {mut(func(r *domain.ReflectionRequest) { r.Language = "fr" }), domain.ErrInvalidReflection},
	}
	for name, c := range cases {
		if err := ValidateReflection(c.req); !errors.Is(err, c.want) && !(err == nil && c.want == nil) {
			t.Errorf("%s: got %v want %v", name, err, c.want)
		}
	}
}
