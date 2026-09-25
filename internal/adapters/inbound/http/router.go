// Package http is the inbound HTTP adapter: routes to the application services.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/handler"
	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

// AdminRoutes are the admin page's dependencies; nil leaves /v1/admin unmounted.
type AdminRoutes struct {
	Auth    inbound.AdminAuthService
	Content inbound.ContentService
	// Origins allowed to call /v1/admin from a browser (the admin page).
	Origins []string
}

func NewRouter(auth inbound.AuthService, account inbound.AccountService, decision inbound.DecisionService, admin *AdminRoutes, log *slog.Logger) http.Handler {
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
	if admin != nil {
		h := handler.NewAdminHandler(admin.Auth, admin.Content, log)
		r.Route("/v1/admin", func(r chi.Router) {
			r.Use(cors.Handler(cors.Options{
				AllowedOrigins: admin.Origins,
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders: []string{"Authorization", "Content-Type"},
				MaxAge:         600,
			}))
			r.Post("/auth/signin", h.SignIn)
			r.Post("/auth/new-password", h.NewPassword)
			r.Post("/auth/refresh", h.Refresh)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AdminMiddleware(admin.Auth))
				r.Get("/me", h.Me)
				r.Get("/collections", h.Collections)
				r.Get("/content/{collection}/{lang}", h.List)
				r.Put("/content/{collection}/{lang}/{id}", h.Save)
				r.Delete("/content/{collection}/{lang}/{id}", h.Delete)
				r.Post("/publish", h.Publish)
				r.Get("/releases", h.Releases)
				r.Get("/counts", h.Counts)
			})
		})
	}
	return r
}
