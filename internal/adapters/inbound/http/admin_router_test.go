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

type fakeAdminAuth struct{}

func (fakeAdminAuth) SignIn(_ context.Context, email, password string) (domain.AdminSession, error) {
	switch {
	case email == "new@missale.app" && password == "temporary":
		return domain.AdminSession{NewPasswordNeeded: true, Session: "s"}, nil
	case password == "right":
		return domain.AdminSession{AccessToken: "admin-token", RefreshToken: "r", ExpiresIn: 3600}, nil
	}
	return domain.AdminSession{}, errors.New("NotAuthorizedException")
}
func (fakeAdminAuth) RespondNewPassword(_ context.Context, _, session, _ string) (domain.AdminSession, error) {
	if session != "s" {
		return domain.AdminSession{}, errors.New("bad session")
	}
	return domain.AdminSession{AccessToken: "admin-token"}, nil
}
func (fakeAdminAuth) Refresh(context.Context, string) (domain.AdminSession, error) {
	return domain.AdminSession{AccessToken: "admin-token"}, nil
}
func (fakeAdminAuth) Verify(_ context.Context, token string) (domain.Admin, []string, error) {
	switch token {
	case "admin-token":
		return domain.Admin{Username: "a@missale.app", Email: "a@missale.app"}, []string{"admin"}, nil
	case "editor-token":
		return domain.Admin{Username: "e@missale.app", Email: "e@missale.app"}, nil, nil
	}
	return domain.Admin{}, nil, errors.New("bad token")
}

type nopContent struct{}

func (nopContent) Collections() []domain.Collection { return domain.Collections }
func (nopContent) List(context.Context, string, string) ([]domain.ContentItem, error) {
	return []domain.ContentItem{}, nil
}
func (nopContent) Save(_ context.Context, by domain.Admin, c, l, id string, data json.RawMessage, p int) (domain.ContentItem, error) {
	return domain.ContentItem{Collection: c, Lang: l, ID: id, Data: data, UpdatedBy: by.Email}, nil
}
func (nopContent) Delete(context.Context, domain.Admin, string, string, string) error { return nil }
func (nopContent) Publish(_ context.Context, by domain.Admin) (domain.Release, error) {
	return domain.Release{Version: 1, PublishedBy: by.Email}, nil
}
func (nopContent) Releases(context.Context) ([]domain.Release, error) { return nil, nil }

func adminRouter() http.Handler {
	return NewRouter(
		service.NewAuthService(fakeIdentity{}, &fakeAuth{}, &fakeUsers{}),
		service.NewAccountService(&fakeAuth{}, &fakeUsers{}),
		service.NewDecisionService(fakeSubs{}, &fakeUsage{}, &fakeJev{}, 5, 2),
		&AdminRoutes{Auth: service.NewAdminAuthService(fakeAdminAuth{}), Content: nopContent{},
			Origins: []string{"https://admin.missale.app"}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func TestAdminSignInAndFirstPassword(t *testing.T) {
	r := adminRouter()
	if rec, out := do(t, r, "POST", "/v1/admin/auth/signin", `{"email":"a@missale.app","password":"wrong"}`, nil); rec.Code != 401 || out["error"] != "invalid_credentials" {
		t.Fatalf("wrong password: %d %v", rec.Code, out)
	}
	rec, out := do(t, r, "POST", "/v1/admin/auth/signin", `{"email":"new@missale.app","password":"temporary"}`, nil)
	if rec.Code != 200 || out["newPasswordNeeded"] != true {
		t.Fatalf("first sign-in should ask for a new password: %d %v", rec.Code, out)
	}
	if rec, _ := do(t, r, "POST", "/v1/admin/auth/new-password", `{"email":"new@missale.app","session":"s","newPassword":"short"}`, nil); rec.Code != 401 {
		t.Fatalf("short new password accepted: %d", rec.Code)
	}
	if rec, out := do(t, r, "POST", "/v1/admin/auth/new-password", `{"email":"new@missale.app","session":"s","newPassword":"a-long-enough-one"}`, nil); rec.Code != 200 || out["accessToken"] != "admin-token" {
		t.Fatalf("new password: %d %v", rec.Code, out)
	}
}

func TestAdminRoutesNeedTheAdminGroup(t *testing.T) {
	r := adminRouter()
	if rec, _ := do(t, r, "GET", "/v1/admin/collections", "", nil); rec.Code != 401 {
		t.Fatalf("no token: %d", rec.Code)
	}
	if rec, _ := do(t, r, "GET", "/v1/admin/collections", "", map[string]string{"Authorization": "Bearer editor-token"}); rec.Code != 403 {
		t.Fatalf("outside the admin group: %d", rec.Code)
	}
	// An app session (Sign in with Apple) must not open the admin page.
	if rec, _ := do(t, r, "GET", "/v1/admin/collections", "", map[string]string{"Authorization": "Bearer access"}); rec.Code != 401 {
		t.Fatalf("app token on admin route: %d", rec.Code)
	}
	rec, out := do(t, r, "GET", "/v1/admin/collections", "", map[string]string{"Authorization": "Bearer admin-token"})
	if rec.Code != 200 || out["collections"] == nil {
		t.Fatalf("admin: %d %v", rec.Code, out)
	}
	rec, out = do(t, r, "PUT", "/v1/admin/content/word_of_day/pt/mateus-5-3-pt", `{"data":{"id":"mateus-5-3-pt"}}`,
		map[string]string{"Authorization": "Bearer admin-token"})
	if rec.Code != 200 || out["updatedBy"] != "a@missale.app" {
		t.Fatalf("save records who edited: %d %v", rec.Code, out)
	}
}

func TestAdminCORSAllowsOnlyTheAdminPage(t *testing.T) {
	r := adminRouter()
	preflight := func(origin string) string {
		req := httptest.NewRequest("OPTIONS", "/v1/admin/collections", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", "GET")
		req.Header.Set("Access-Control-Request-Headers", "authorization")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Header().Get("Access-Control-Allow-Origin")
	}
	if got := preflight("https://admin.missale.app"); got != "https://admin.missale.app" {
		t.Fatalf("admin page origin not allowed: %q", got)
	}
	if got := preflight("https://evil.example"); strings.Contains(got, "evil") {
		t.Fatalf("foreign origin allowed: %q", got)
	}
}
