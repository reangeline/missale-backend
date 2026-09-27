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

type deleteAccountDTO struct {
	// AuthorizationCode is a fresh Sign in with Apple code, asked for right
	// before this call, so we can revoke the Apple token (guideline
	// 5.1.1(v)). Empty for app versions that don't send it yet.
	AuthorizationCode string `json:"authorizationCode"`
}

func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	var body deleteAccountDTO
	// Older app versions call DELETE with no body at all; only decode when
	// there is one, so they keep working unchanged.
	if r.ContentLength > 0 {
		if !decodeJSON(w, r, &body) {
			RespondError(w, http.StatusBadRequest, "invalid_request")
			return
		}
	}
	if err := h.accountService.DeleteAccount(r.Context(), middleware.PrincipalFrom(r), body.AuthorizationCode); err != nil {
		h.log.Error("delete account", "err", err)
		respondDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
