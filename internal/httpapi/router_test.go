package httpapi

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

	"github.com/reangeline/missale-backend/internal/auth"
	"github.com/reangeline/missale-backend/internal/jev"
	"github.com/reangeline/missale-backend/internal/subscription"
)

type fakeApple struct{}

func (fakeApple) Verify(_ context.Context, token string) (auth.AppleIdentity, error) {
	if token != "good-apple" {
		return auth.AppleIdentity{}, errors.New("bad")
	}
	return auth.AppleIdentity{Sub: "001.abc"}, nil
}

type fakeSessions struct{ deleted []string }

func (f *fakeSessions) SignIn(_ context.Context, a auth.AppleIdentity) (string, auth.Tokens, error) {
	return "user-1", auth.Tokens{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, nil
}
func (f *fakeSessions) Refresh(_ context.Context, rt string) (auth.Tokens, error) {
	if rt != "refresh" {
		return auth.Tokens{}, errors.New("expired")
	}
	return auth.Tokens{AccessToken: "access2", ExpiresIn: 3600}, nil
}
func (f *fakeSessions) Delete(_ context.Context, username string) error {
	f.deleted = append(f.deleted, username)
	return nil
}
func (f *fakeSessions) Verify(_ context.Context, token string) (auth.Principal, error) {
	if token != "access" {
		return auth.Principal{}, errors.New("bad")
	}
	return auth.Principal{UserID: "user-1", Username: "apple_001.abc"}, nil
}

type fakeStore struct {
	touched []string
	calls   int
	deleted []string
}

func (f *fakeStore) TouchUser(_ context.Context, id, _ string) error {
	f.touched = append(f.touched, id)
	return nil
}
func (f *fakeStore) ReserveDecision(_ context.Context, _ string, limit int) (bool, error) {
	f.calls++
	return f.calls <= limit, nil
}
func (f *fakeStore) DeleteUser(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeSubs struct{}

func (fakeSubs) Verify(jws string) (subscription.Transaction, error) {
	if jws != "subscribed" {
		return subscription.Transaction{}, subscription.ErrNotSubscribed
	}
	return subscription.Transaction{ProductID: "anual"}, nil
}

type fakeJev struct{ states []string }

func (f *fakeJev) Decide(_ context.Context, state string, _ map[string]jev.Question) (json.RawMessage, error) {
	f.states = append(f.states, state)
	return json.RawMessage(`{"risk":{"type":"noul","noul":0.02}}`), nil
}

func newAPI() (*API, *fakeSessions, *fakeStore, *fakeJev) {
	s, st, j := &fakeSessions{}, &fakeStore{}, &fakeJev{}
	return &API{Apple: fakeApple{}, Sessions: s, Store: st, Subscriptions: fakeSubs{}, Jev: j, DailyLimit: 2,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, s, st, j
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
	api, _, store, _ := newAPI()
	rec, out := do(t, api.Router(), "POST", "/v1/auth/apple", `{"identityToken":"good-apple"}`, nil)
	if rec.Code != 200 || out["accessToken"] != "access" || out["refreshToken"] != "refresh" {
		t.Fatalf("sign in: %d %v", rec.Code, out)
	}
	if len(store.touched) != 1 || store.touched[0] != "user-1" {
		t.Fatalf("user not recorded: %v", store.touched)
	}
	if rec, _ := do(t, api.Router(), "POST", "/v1/auth/apple", `{"identityToken":"forged"}`, nil); rec.Code != 401 {
		t.Fatalf("forged Apple token: %d", rec.Code)
	}
}

func TestRefresh(t *testing.T) {
	api, _, _, _ := newAPI()
	if rec, out := do(t, api.Router(), "POST", "/v1/auth/refresh", `{"refreshToken":"refresh"}`, nil); rec.Code != 200 || out["accessToken"] != "access2" {
		t.Fatalf("refresh: %d %v", rec.Code, out)
	}
	if rec, _ := do(t, api.Router(), "POST", "/v1/auth/refresh", `{"refreshToken":"old"}`, nil); rec.Code != 401 {
		t.Fatalf("expired refresh: %d", rec.Code)
	}
}

func TestDecisionsRequireSessionAndSubscription(t *testing.T) {
	api, _, _, j := newAPI()
	r := api.Router()
	if rec, _ := do(t, r, "POST", "/v1/decisions", validDecision, nil); rec.Code != 401 {
		t.Fatalf("no session: %d", rec.Code)
	}
	if rec, _ := do(t, r, "POST", "/v1/decisions", validDecision, map[string]string{"Authorization": "Bearer access"}); rec.Code != 402 {
		t.Fatalf("no subscription: %d", rec.Code)
	}
	rec, out := do(t, r, "POST", "/v1/decisions", validDecision, authed)
	if rec.Code != 200 || out["answers"] == nil {
		t.Fatalf("decision: %d %v", rec.Code, out)
	}
	if len(j.states) != 1 {
		t.Fatalf("Jev called %d times", len(j.states))
	}
}

func TestDecisionsDailyLimit(t *testing.T) {
	api, _, _, _ := newAPI() // limit 2
	r := api.Router()
	for i := 0; i < 2; i++ {
		if rec, _ := do(t, r, "POST", "/v1/decisions", validDecision, authed); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	if rec, out := do(t, r, "POST", "/v1/decisions", validDecision, authed); rec.Code != 429 || out["error"] != "daily_limit" {
		t.Fatalf("over limit: %d %v", rec.Code, out)
	}
}

func TestDecisionsRejectOversizedOrMalformedRequests(t *testing.T) {
	api, _, store, j := newAPI()
	r := api.Router()
	long := strings.Repeat("a", maxStateChars+1)
	many := `{"a":"x"`
	for i := 0; i < maxCriteria; i++ {
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
		if rec, _ := do(t, r, "POST", "/v1/decisions", body, authed); rec.Code != 400 {
			t.Errorf("%s: expected 400, got %d", name, rec.Code)
		}
	}
	if store.calls != 0 || len(j.states) != 0 {
		t.Fatalf("invalid requests reached usage (%d) or Jev (%d)", store.calls, len(j.states))
	}
}

func TestDeleteAccount(t *testing.T) {
	api, sessions, store, _ := newAPI()
	rec, _ := do(t, api.Router(), "DELETE", "/v1/account", "", map[string]string{"Authorization": "Bearer access"})
	if rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "user-1" || len(sessions.deleted) != 1 || sessions.deleted[0] != "apple_001.abc" {
		t.Fatalf("not fully deleted: rows %v, cognito %v", store.deleted, sessions.deleted)
	}
}
