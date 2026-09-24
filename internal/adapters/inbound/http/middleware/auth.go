package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type contextKey struct{}

// AuthMiddleware requires a valid Cognito access token (Bearer).
func AuthMiddleware(authService inbound.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				unauthorized(w)
				return
			}
			p, err := authService.Authenticate(r.Context(), token)
			if err != nil {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
		})
	}
}

// PrincipalFrom returns the authenticated user set by AuthMiddleware.
func PrincipalFrom(r *http.Request) domain.Principal {
	p, _ := r.Context().Value(contextKey{}).(domain.Principal)
	return p
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": domain.ErrUnauthorized.Error()})
}
