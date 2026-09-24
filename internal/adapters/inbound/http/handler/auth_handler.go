package handler

import (
	"log/slog"
	"net/http"

	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

type AuthHandler struct {
	authService inbound.AuthService
	log         *slog.Logger
}

func NewAuthHandler(authService inbound.AuthService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, log: log}
}

type signInWithAppleDTO struct {
	IdentityToken string `json:"identityToken"`
}

type refreshDTO struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) SignInWithApple(w http.ResponseWriter, r *http.Request) {
	var body signInWithAppleDTO
	if !decodeJSON(w, r, &body) || body.IdentityToken == "" {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	session, err := h.authService.SignInWithApple(r.Context(), body.IdentityToken)
	if err != nil {
		if respondDomainError(w, err) >= 500 {
			h.log.Error("sign in with apple", "err", err)
		}
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshDTO
	if !decodeJSON(w, r, &body) || body.RefreshToken == "" {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	session, err := h.authService.RefreshSession(r.Context(), body.RefreshToken)
	if err != nil {
		if respondDomainError(w, err) >= 500 {
			h.log.Error("refresh", "err", err)
		}
		return
	}
	respondJSON(w, http.StatusOK, session)
}
