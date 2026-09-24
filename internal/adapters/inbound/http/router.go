// Package http is the inbound HTTP adapter: routes to the application services.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/handler"
	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

func NewRouter(auth inbound.AuthService, account inbound.AccountService, decision inbound.DecisionService, log *slog.Logger) http.Handler {
	authHandler := handler.NewAuthHandler(auth, log)
	accountHandler := handler.NewAccountHandler(account, log)
	decisionHandler := handler.NewDecisionHandler(decision, log)

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Post("/v1/auth/apple", authHandler.SignInWithApple)
	r.Post("/v1/auth/refresh", authHandler.Refresh)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(auth))
		r.Delete("/v1/account", accountHandler.Delete)
		r.Post("/v1/decisions", decisionHandler.Decide)
	})
	return r
}
