package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

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
	start := time.Now()
	var req domain.DecisionRequest
	if !decodeJSON(w, r, &req) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		logUse(h.log, "decision", r, middleware.PrincipalFrom(r).UserID, http.StatusBadRequest, start, nil)
		return
	}
	p := middleware.PrincipalFrom(r)
	answers, err := h.decisionService.Decide(r.Context(), p, r.Header.Get("X-Subscription"), req)
	if err != nil {
		logUse(h.log, "decision", r, p.UserID, respondDomainError(w, err), start, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]json.RawMessage{"answers": answers})
	logUse(h.log, "decision", r, p.UserID, http.StatusOK, start, nil)
}
