// Package httpapi is the Missale API: Sign in with Apple, session refresh,
// account deletion, and the Jev proxy behind login + subscription.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/reangeline/missale-backend/internal/auth"
	"github.com/reangeline/missale-backend/internal/jev"
	"github.com/reangeline/missale-backend/internal/subscription"
)

type AppleVerifier interface {
	Verify(ctx context.Context, identityToken string) (auth.AppleIdentity, error)
}

type Sessions interface {
	SignIn(ctx context.Context, apple auth.AppleIdentity) (string, auth.Tokens, error)
	Refresh(ctx context.Context, refreshToken string) (auth.Tokens, error)
	Delete(ctx context.Context, username string) error
	Verify(ctx context.Context, accessToken string) (auth.Principal, error)
}

type Store interface {
	TouchUser(ctx context.Context, userID, appleSub string) error
	ReserveDecision(ctx context.Context, userID string, limit int) (bool, error)
	DeleteUser(ctx context.Context, userID string) error
}

type SubscriptionVerifier interface {
	Verify(jws string) (subscription.Transaction, error)
}

type Decider interface {
	Decide(ctx context.Context, state string, questions map[string]jev.Question) (json.RawMessage, error)
}

type API struct {
	Apple         AppleVerifier
	Sessions      Sessions
	Store         Store
	Subscriptions SubscriptionVerifier
	Jev           Decider
	DailyLimit    int
	Log           *slog.Logger
}

// Limits on what the app may forward to Jev, so the proxy can't be used as a
// general-purpose, unbounded Jev endpoint.
const (
	maxStateChars        = 2000
	maxQuestions         = 3
	maxInstructionsChars = 300
	maxCriteria          = 32
	maxCriterionChars    = 400
	maxBodyBytes         = 64 << 10
)

func (a *API) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/v1/auth/apple", a.signInWithApple)
	r.Post("/v1/auth/refresh", a.refresh)
	r.Group(func(r chi.Router) {
		r.Use(a.requireSession)
		r.Delete("/v1/account", a.deleteAccount)
		r.Post("/v1/decisions", a.decide)
	})
	return r
}

type principalKey struct{}

func (a *API) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		p, err := a.Sessions.Verify(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}

func principal(r *http.Request) auth.Principal {
	p, _ := r.Context().Value(principalKey{}).(auth.Principal)
	return p
}

func (a *API) signInWithApple(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IdentityToken string `json:"identityToken"`
	}
	if !readJSON(w, r, &body) || body.IdentityToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	identity, err := a.Apple.Verify(r.Context(), body.IdentityToken)
	if err != nil {
		a.Log.Info("apple token rejected", "err", err)
		writeError(w, http.StatusUnauthorized, "invalid_apple_token")
		return
	}
	userID, tokens, err := a.Sessions.SignIn(r.Context(), identity)
	if err != nil {
		a.Log.Error("sign in", "err", err)
		writeError(w, http.StatusBadGateway, "sign_in_failed")
		return
	}
	if err := a.Store.TouchUser(r.Context(), userID, identity.Sub); err != nil {
		a.Log.Error("touch user", "err", err)
		writeError(w, http.StatusInternalServerError, "sign_in_failed")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !readJSON(w, r, &body) || body.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	tokens, err := a.Sessions.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "session_expired")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if err := a.Store.DeleteUser(r.Context(), p.UserID); err != nil {
		a.Log.Error("delete user rows", "err", err)
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	if err := a.Sessions.Delete(r.Context(), p.Username); err != nil {
		a.Log.Error("delete cognito user", "err", err)
		writeError(w, http.StatusBadGateway, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type decisionRequest struct {
	State     string                  `json:"state"`
	Questions map[string]jev.Question `json:"questions"`
}

func (a *API) decide(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	if _, err := a.Subscriptions.Verify(r.Header.Get("X-Subscription")); err != nil {
		a.Log.Info("subscription rejected", "user", p.UserID, "err", err)
		writeError(w, http.StatusPaymentRequired, "subscription_required")
		return
	}
	var req decisionRequest
	if !readJSON(w, r, &req) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := validate(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	allowed, err := a.Store.ReserveDecision(r.Context(), p.UserID, a.DailyLimit)
	if err != nil {
		a.Log.Error("reserve decision", "err", err)
		writeError(w, http.StatusInternalServerError, "usage_unavailable")
		return
	}
	if !allowed {
		writeError(w, http.StatusTooManyRequests, "daily_limit")
		return
	}
	// The state is what the user wrote: it goes to Jev and nowhere else —
	// not to logs, not to the database.
	answers, err := a.Jev.Decide(r.Context(), req.State, req.Questions)
	if err != nil {
		a.Log.Error("jev", "user", p.UserID, "err", err)
		writeError(w, http.StatusBadGateway, "jev_unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"answers": answers})
}

func validate(req decisionRequest) error {
	if strings.TrimSpace(req.State) == "" || utf8.RuneCountInString(req.State) > maxStateChars {
		return errors.New("invalid_state")
	}
	if len(req.Questions) == 0 || len(req.Questions) > maxQuestions {
		return errors.New("invalid_questions")
	}
	for _, q := range req.Questions {
		if q.Instructions == "" || utf8.RuneCountInString(q.Instructions) > maxInstructionsChars {
			return errors.New("invalid_questions")
		}
		switch q.Type {
		case "noul":
			if len(q.Criteria) != 0 {
				return errors.New("invalid_questions")
			}
		case "choice":
			var criteria map[string]string
			if json.Unmarshal(q.Criteria, &criteria) != nil || !criteriaFit(len(criteria), mapValues(criteria)) {
				return errors.New("invalid_questions")
			}
		case "score":
			var levels []string
			if json.Unmarshal(q.Criteria, &levels) != nil || !criteriaFit(len(levels), levels) {
				return errors.New("invalid_questions")
			}
		default:
			return errors.New("invalid_questions")
		}
	}
	return nil
}

func criteriaFit(n int, texts []string) bool {
	if n < 2 || n > maxCriteria {
		return false
	}
	for _, t := range texts {
		if t == "" || utf8.RuneCountInString(t) > maxCriterionChars {
			return false
		}
	}
	return true
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if k == "" {
			return []string{""}
		}
		out = append(out, v)
	}
	return out
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(v) == nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
