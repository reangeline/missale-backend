package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type adminKey struct{}

// AdminMiddleware requires an admin-client access token from a member of the
// admin group: 401 without a valid token, 403 for anyone else.
func AdminMiddleware(auth inbound.AdminAuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				writeErr(w, http.StatusUnauthorized, domain.ErrUnauthorized)
				return
			}
			admin, err := auth.Authenticate(r.Context(), token)
			if errors.Is(err, domain.ErrForbidden) {
				writeErr(w, http.StatusForbidden, domain.ErrForbidden)
				return
			}
			if err != nil {
				writeErr(w, http.StatusUnauthorized, domain.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminKey{}, admin)))
		})
	}
}

// AdminFrom returns the admin set by AdminMiddleware.
func AdminFrom(r *http.Request) domain.Admin {
	a, _ := r.Context().Value(adminKey{}).(domain.Admin)
	return a
}

func writeErr(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
