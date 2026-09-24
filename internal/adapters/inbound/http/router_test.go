package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/reangeline/missale-backend/internal/application/service"
	"github.com/reangeline/missale-backend/internal/core/domain"
)

// Real application services behind the router; only the outbound ports are fake.

type fakeIdentity struct{}

func (fakeIdentity) Verify(_ context.Context, token string) (domain.AppleIdentity, error) {
	if token != "good-apple" {
		return domain.AppleIdentity{}, errors.New("bad")
	}
	return domain.AppleIdentity{Sub: "001.abc"}, nil
}

type fakeAuth struct{ deleted []string }

func (f *fakeAuth) SignIn(_ context.Context, _ domain.AppleIdentity) (string, domain.Session, error) {
	return "user-1", domain.Session{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, nil
}
func (f *fakeAuth) Refresh(_ context.Context, rt string) (domain.Session, error) {
	if rt != "refresh" {
		return domain.Session{}, errors.New("expired")
	}
	return domain.Session{AccessToken: "access2", ExpiresIn: 3600}, nil
}
func (f *fakeAuth) Delete(_ context.Context, username string) error {
	f.deleted = append(f.deleted, username)
	return nil
}
func (f *fakeAuth) Verify(_ context.Context, token string) (domain.Principal, error) {
	if token != "access" {
		return domain.Principal{}, errors.New("bad")
	}
	return domain.Principal{UserID: "user-1", Username: "apple_001.abc"}, nil
}

type fakeUsers struct{ saved, deleted []string }

func (f *fakeUsers) Save(_ context.Context, u domain.User) error {
	f.saved = append(f.saved, u.ID)
	return nil
}
func (f *fakeUsers) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeUsage struct{ calls int }

func (f *fakeUsage) Reserve(context.Context, string) (int, error) {
	f.calls++
	return f.calls, nil
}

type fakeSubs struct{}

func (fakeSubs) Verify(jws string) (domain.SubscriptionTransaction, error) {
	if jws != "subscribed" {
		return domain.SubscriptionTransaction{}, domain.ErrNotSubscribed
	}
	return domain.SubscriptionTransaction{ProductID: "anual"}, nil
}

type fakeJev struct{ states []string }

func (f *fakeJev) Decide(_ context.Context, state string, _ map[string]domain.Question) (json.RawMessage, error) {
	f.states = append(f.states, state)
	return json.RawMessage(`{"risk":{"type":"noul","noul":0.02}}`), nil
}

type fixture struct {
	router http.Handler
	auth   *fakeAuth
	users  *fakeUsers
	usage  *fakeUsage
	jev    *fakeJev
}

func newFixture() fixture {
	f := fixture{auth: &fakeAuth{}, users: &fakeUsers{}, usage: &fakeUsage{}, jev: &fakeJev{}}
	f.router = NewRouter(
		service.NewAuthService(fakeIdentity{}, f.auth, f.users),
		service.NewAccountService(f.auth, f.users),
		service.NewDecisionService(fakeSubs{}, f.usage, f.jev, 2),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	return f
}

func do(t *testing.T, h http.Handler, method, path, body string, headers map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

const validDecision = `{"state":"Hoje faz um ano que perdi meu bebê.","questions":{
	"risk":{"type":"noul","instructions":"Does the person express a wish to end their own life?"},
	"state":{"type":"choice","instructions":"Which state?","criteria":{"grief":"loss","lonely":"alone"}},
	"pain":{"type":"score","instructions":"How intense?","criteria":["none","mild","strong"]}}}`

var authed = map[string]string{"Authorization": "Bearer access", "X-Subscription": "subscribed"}

func TestSignInWithApple(t *testing.T) {
	f := newFixture()
	rec, out := do(t, f.router, "POST", "/v1/auth/apple", `{"identityToken":"good-apple"}`, nil)
	if rec.Code != 200 || out["accessToken"] != "access" || out["refreshToken"] != "refresh" {
		t.Fatalf("sign in: %d %v", rec.Code, out)
	}
	if len(f.users.saved) != 1 || f.users.saved[0] != "user-1" {
		t.Fatalf("user not recorded: %v", f.users.saved)
	}
	if rec, out := do(t, f.router, "POST", "/v1/auth/apple", `{"identityToken":"forged"}`, nil); rec.Code != 401 || out["error"] != "invalid_apple_token" {
		t.Fatalf("forged Apple token: %d %v", rec.Code, out)
	}
}

func TestRefresh(t *testing.T) {
	f := newFixture()
	if rec, out := do(t, f.router, "POST", "/v1/auth/refresh", `{"refreshToken":"refresh"}`, nil); rec.Code != 200 || out["accessToken"] != "access2" {
		t.Fatalf("refresh: %d %v", rec.Code, out)
	}
	if rec, out := do(t, f.router, "POST", "/v1/auth/refresh", `{"refreshToken":"old"}`, nil); rec.Code != 401 || out["error"] != "session_expired" {
		t.Fatalf("expired refresh: %d %v", rec.Code, out)
	}
}

func TestDecisionsRequireSessionAndSubscription(t *testing.T) {
	f := newFixture()
	if rec, _ := do(t, f.router, "POST", "/v1/decisions", validDecision, nil); rec.Code != 401 {
		t.Fatalf("no session: %d", rec.Code)
	}
	if rec, out := do(t, f.router, "POST", "/v1/decisions", validDecision, map[string]string{"Authorization": "Bearer access"}); rec.Code != 402 || out["error"] != "subscription_required" {
		t.Fatalf("no subscription: %d %v", rec.Code, out)
	}
	rec, out := do(t, f.router, "POST", "/v1/decisions", validDecision, authed)
	if rec.Code != 200 || out["answers"] == nil {
		t.Fatalf("decision: %d %v", rec.Code, out)
	}
	if len(f.jev.states) != 1 {
		t.Fatalf("Jev called %d times", len(f.jev.states))
	}
}

func TestDecisionsDailyLimit(t *testing.T) {
	f := newFixture() // limit 2
	for i := 0; i < 2; i++ {
		if rec, _ := do(t, f.router, "POST", "/v1/decisions", validDecision, authed); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	if rec, out := do(t, f.router, "POST", "/v1/decisions", validDecision, authed); rec.Code != 429 || out["error"] != "daily_limit" {
		t.Fatalf("over limit: %d %v", rec.Code, out)
	}
}

func TestDecisionsRejectOversizedOrMalformedRequests(t *testing.T) {
	f := newFixture()
	long := strings.Repeat("a", domain.MaxStateChars+1)
	many := `{"a":"x"`
	for i := 0; i < domain.MaxCriteria; i++ {
		many += `,"k` + strings.Repeat("x", i+1) + `":"x"`
	}
	many += `}`
	bad := map[string]string{
		"empty state":    `{"state":" ","questions":{"r":{"type":"noul","instructions":"q"}}}`,
		"state too long": `{"state":"` + long + `","questions":{"r":{"type":"noul","instructions":"q"}}}`,
		"no questions":   `{"state":"oi","questions":{}}`,
		"too many":       `{"state":"oi","questions":{"a":{"type":"noul","instructions":"q"},"b":{"type":"noul","instructions":"q"},"c":{"type":"noul","instructions":"q"},"d":{"type":"noul","instructions":"q"}}}`,
		"unknown type":   `{"state":"oi","questions":{"a":{"type":"text","instructions":"write a poem"}}}`,
		"one option":     `{"state":"oi","questions":{"a":{"type":"choice","instructions":"q","criteria":{"x":"y"}}}}`,
		"33 options":     `{"state":"oi","questions":{"a":{"type":"choice","instructions":"q","criteria":` + many + `}}}`,
		"model override": `{"state":"oi","model":"gpt","questions":{"a":{"type":"noul","instructions":"q"}}}`,
	}
	for name, body := range bad {
		if rec, _ := do(t, f.router, "POST", "/v1/decisions", body, authed); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	if f.usage.calls != 0 || len(f.jev.states) != 0 {
		t.Fatalf("invalid requests reached usage (%d) or Jev (%d)", f.usage.calls, len(f.jev.states))
	}
}

func TestDeleteAccount(t *testing.T) {
	f := newFixture()
	rec, _ := do(t, f.router, "DELETE", "/v1/account", "", map[string]string{"Authorization": "Bearer access"})
	if rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if len(f.users.deleted) != 1 || f.users.deleted[0] != "user-1" || len(f.auth.deleted) != 1 || f.auth.deleted[0] != "apple_001.abc" {
		t.Fatalf("not fully deleted: rows %v, cognito %v", f.users.deleted, f.auth.deleted)
	}
}
