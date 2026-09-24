package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type DecisionHandler struct {
	decisionService inbound.DecisionService
	log             *slog.Logger
}

func NewDecisionHandler(decisionService inbound.DecisionService, log *slog.Logger) *DecisionHandler {
	return &DecisionHandler{decisionService: decisionService, log: log}
}

// Decide takes the StoreKit transaction JWS in the X-Subscription header.
// The body's state is what the user wrote: it is never logged.
func (h *DecisionHandler) Decide(w http.ResponseWriter, r *http.Request) {
	var req domain.DecisionRequest
	if !decodeJSON(w, r, &req) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	p := middleware.PrincipalFrom(r)
	answers, err := h.decisionService.Decide(r.Context(), p, r.Header.Get("X-Subscription"), req)
	if err != nil {
		if status := respondDomainError(w, err); status >= 500 || status == http.StatusPaymentRequired {
			h.log.Warn("decision", "user", p.UserID, "status", status, "err", err)
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]json.RawMessage{"answers": answers})
}
