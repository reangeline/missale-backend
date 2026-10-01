package http

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/reangeline/missale-backend/internal/application/service"
	"github.com/reangeline/missale-backend/internal/core/domain"
)

type fakeWriter struct {
	reqs []domain.ReflectionRequest
	err  error
}

func (f *fakeWriter) Write(_ context.Context, r domain.ReflectionRequest) (string, error) {
	f.reqs = append(f.reqs, r)
	if f.err != nil {
		return "", f.err
	}
	return "Que a paz de Deus te acompanhe.", nil
}

const validReflection = `{"state":"Estou cansado.","passage":{"reference":"Mt 11,28","text":"Vinde a mim."},"saint":{"name":"Santa Mônica","summary":"Rezou anos por seu filho."},"language":"pt"}`

func TestReflectionsRequireASession(t *testing.T) {
	f := newFixture()
	if rec, _ := do(t, f.router, "POST", "/v1/reflections", validReflection, nil); rec.Code != 401 {
		t.Fatalf("no session: %d", rec.Code)
	}
}

func TestReflectionsReturnTheReflectionAndCountOneUse(t *testing.T) {
	f := newFixture()
	rec, out := do(t, f.router, "POST", "/v1/reflections", validReflection, authed)
	if rec.Code != 200 || out["reflection"] != "Que a paz de Deus te acompanhe." {
		t.Fatalf("reflection: %d %v", rec.Code, out)
	}
	if f.usage.calls != 1 || f.usage.free != 0 || len(f.writer.reqs) != 1 {
		t.Fatalf("calls %d, free %d, writer %d", f.usage.calls, f.usage.free, len(f.writer.reqs))
	}
}

func TestReflectionsAcceptTheOptionalContext(t *testing.T) {
	f := newFixture()
	b := strings.Replace(validReflection, `"language"`, `"context":"Como reza: pouco\nFé: buscando","language"`, 1)
	if rec, _ := do(t, f.router, "POST", "/v1/reflections", b, authed); rec.Code != 200 {
		t.Fatalf("context: %d", rec.Code)
	}
	if len(f.writer.reqs) != 1 || f.writer.reqs[0].Context != "Como reza: pouco\nFé: buscando" {
		t.Fatalf("writer got %+v", f.writer.reqs)
	}
}

func TestReflectionsFreeAllowanceRules(t *testing.T) {
	noSub := map[string]string{"Authorization": "Bearer access"}
	f := newFixture() // free allowance 3 for reflections' service
	freeFalse := strings.Replace(validReflection, `"language":"pt"`, `"language":"pt","free":false`, 1)
	if rec, out := do(t, f.router, "POST", "/v1/reflections", freeFalse, noSub); rec.Code != 402 || out["error"] != "subscription_required" {
		t.Fatalf("free:false: %d %v", rec.Code, out)
	}
	if f.usage.calls != 0 || len(f.writer.reqs) != 0 {
		t.Fatalf("free:false spent: calls %d, writer %d", f.usage.calls, len(f.writer.reqs))
	}
	if rec, _ := do(t, f.router, "POST", "/v1/reflections", validReflection, noSub); rec.Code != 200 {
		t.Fatalf("free: %d", rec.Code)
	}
	if f.usage.free != 1 {
		t.Fatalf("free spent %d, want 1", f.usage.free)
	}
}

func TestReflectionsWriterFailureIs502(t *testing.T) {
	f := newFixture()
	f.writer.err = errors.New("claude: refusal (bio)")
	rec, out := do(t, f.router, "POST", "/v1/reflections", validReflection, authed)
	if rec.Code != 502 || out["error"] != "reflection_unavailable" {
		t.Fatalf("writer failure: %d %v", rec.Code, out)
	}
}

func TestReflectionsWithoutAnthropicKeyAre503(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	usage := &fakeUsage{}
	r := NewRouter(
		service.NewAuthService(fakeIdentity{}, &fakeAuth{}, &fakeUsers{}),
		service.NewAccountService(&fakeAuth{}, &fakeUsers{}, &fakeRevoker{}, log),
		service.NewDecisionService(fakeSubs{}, usage, &fakeJev{}, 5, 3),
		service.NewReflectionService(fakeSubs{}, usage, nil, 5, 3),
		nil, log,
	)
	if rec, out := do(t, r, "POST", "/v1/reflections", validReflection, authed); rec.Code != 503 || out["error"] != "reflection_not_configured" {
		t.Fatalf("no key: %d %v", rec.Code, out)
	}
	if usage.calls != 0 {
		t.Fatalf("503 spent a use")
	}
	// the rest of the API keeps working
	if rec, _ := do(t, r, "POST", "/v1/decisions", validDecision, authed); rec.Code != 200 {
		t.Fatalf("decisions: %d", rec.Code)
	}
}

func TestReflectionsRejectInvalidRequests(t *testing.T) {
	f := newFixture()
	long := func(n int) string { return strings.Repeat("a", n) }
	body := func(state, ref, text, name, summary, lang string) string {
		return `{"state":"` + state + `","passage":{"reference":"` + ref + `","text":"` + text + `"},"saint":{"name":"` + name + `","summary":"` + summary + `"},"language":"` + lang + `"}`
	}
	bad := map[string]string{
		"empty state":      body(" ", "r", "t", "n", "s", "pt"),
		"state too long":   body(long(domain.MaxStateChars+1), "r", "t", "n", "s", "pt"),
		"no reference":     body("oi", "", "t", "n", "s", "pt"),
		"no saint name":    body("oi", "r", "t", " ", "s", "pt"),
		"passage too long": body("oi", "r", long(domain.MaxPassageChars+1), "n", "s", "pt"),
		"summary too long": body("oi", "r", "t", "n", long(domain.MaxSummaryChars+1), "pt"),
		"bad language":     body("oi", "r", "t", "n", "s", "fr"),
		"no language":      body("oi", "r", "t", "n", "s", ""),
		"context too long": strings.Replace(validReflection, `"language"`, `"context":"`+long(domain.MaxContextChars+1)+`","language"`, 1),
		"unknown field":    strings.Replace(validReflection, `"language"`, `"model":"x","language"`, 1),
		"malformed":        `{`,
	}
	for name, b := range bad {
		if rec, _ := do(t, f.router, "POST", "/v1/reflections", b, authed); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	if f.usage.calls != 0 || len(f.writer.reqs) != 0 {
		t.Fatalf("invalid requests reached usage (%d) or the writer (%d)", f.usage.calls, len(f.writer.reqs))
	}
}

func TestReflectionsDailyLimit(t *testing.T) {
	f := newFixture() // limit 5
	for i := 0; i < 5; i++ {
		if rec, _ := do(t, f.router, "POST", "/v1/reflections", validReflection, authed); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	if rec, out := do(t, f.router, "POST", "/v1/reflections", validReflection, authed); rec.Code != 429 || out["error"] != "daily_limit" {
		t.Fatalf("over limit: %d %v", rec.Code, out)
	}
}
