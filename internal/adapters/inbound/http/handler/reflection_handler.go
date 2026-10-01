package handler

import (
	"log/slog"
	"net/http"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type ReflectionHandler struct {
	reflectionService inbound.ReflectionService
	log               *slog.Logger
}

func NewReflectionHandler(reflectionService inbound.ReflectionService, log *slog.Logger) *ReflectionHandler {
	return &ReflectionHandler{reflectionService: reflectionService, log: log}
}

// Reflect takes the StoreKit transaction JWS in the X-Subscription header.
// The body's state and the reflection are never logged.
func (h *ReflectionHandler) Reflect(w http.ResponseWriter, r *http.Request) {
	var req domain.ReflectionRequest
	if !decodeJSON(w, r, &req) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	p := middleware.PrincipalFrom(r)
	text, err := h.reflectionService.Reflect(r.Context(), p, r.Header.Get("X-Subscription"), req)
	if err != nil {
		if status := respondDomainError(w, err); status >= 500 || status == http.StatusPaymentRequired {
			h.log.Warn("reflection", "user", p.UserID, "status", status, "err", err)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"reflection": text})
}
