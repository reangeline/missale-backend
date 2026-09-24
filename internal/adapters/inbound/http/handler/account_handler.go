package handler

import (
	"log/slog"
	"net/http"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type AccountHandler struct {
	accountService inbound.AccountService
	log            *slog.Logger
}

func NewAccountHandler(accountService inbound.AccountService, log *slog.Logger) *AccountHandler {
	return &AccountHandler{accountService: accountService, log: log}
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.accountService.DeleteAccount(r.Context(), middleware.PrincipalFrom(r)); err != nil {
		h.log.Error("delete account", "err", err)
		respondDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
