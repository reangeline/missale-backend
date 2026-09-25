package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/reangeline/missale-backend/internal/adapters/inbound/http/middleware"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
)

// AdminHandler serves the admin page: sign-in, content editing, publishing.
type AdminHandler struct {
	auth    inbound.AdminAuthService
	content inbound.ContentService
	log     *slog.Logger
}

func NewAdminHandler(auth inbound.AdminAuthService, content inbound.ContentService, log *slog.Logger) *AdminHandler {
	return &AdminHandler{auth: auth, content: content, log: log}
}

func (h *AdminHandler) fail(w http.ResponseWriter, action string, err error) {
	if respondDomainError(w, err) >= 500 {
		h.log.Error("admin "+action, "err", err)
	}
}

func (h *AdminHandler) SignIn(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password string }
	if !decodeJSON(w, r, &body) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	session, err := h.auth.SignIn(r.Context(), body.Email, body.Password)
	if err != nil {
		h.fail(w, "sign in", err)
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (h *AdminHandler) NewPassword(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Session, NewPassword string }
	if !decodeJSON(w, r, &body) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	session, err := h.auth.CompleteNewPassword(r.Context(), body.Email, body.Session, body.NewPassword)
	if err != nil {
		h.fail(w, "new password", err)
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (h *AdminHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct{ RefreshToken string }
	if !decodeJSON(w, r, &body) || body.RefreshToken == "" {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	session, err := h.auth.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		h.fail(w, "refresh", err)
		return
	}
	respondJSON(w, http.StatusOK, session)
}

func (h *AdminHandler) Me(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"email": middleware.AdminFrom(r).Email})
}

func (h *AdminHandler) Collections(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"collections": h.content.Collections()})
}

func (h *AdminHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.content.List(r.Context(), chi.URLParam(r, "collection"), chi.URLParam(r, "lang"))
	if err != nil {
		h.fail(w, "list", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *AdminHandler) Save(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Data     json.RawMessage `json:"data"`
		Position *int            `json:"position"`
	}
	if !decodeJSONLimit(w, r, &body, maxAdminBodyBytes) {
		RespondError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	position := -1
	if body.Position != nil {
		position = *body.Position
	}
	item, err := h.content.Save(r.Context(), middleware.AdminFrom(r),
		chi.URLParam(r, "collection"), chi.URLParam(r, "lang"), chi.URLParam(r, "id"), body.Data, position)
	if err != nil {
		h.fail(w, "save", err)
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (h *AdminHandler) Delete(w http.ResponseWriter, r *http.Request) {
	err := h.content.Delete(r.Context(), middleware.AdminFrom(r),
		chi.URLParam(r, "collection"), chi.URLParam(r, "lang"), chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, "delete", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) Publish(w http.ResponseWriter, r *http.Request) {
	release, err := h.content.Publish(r.Context(), middleware.AdminFrom(r))
	if err != nil {
		h.fail(w, "publish", err)
		return
	}
	h.log.Info("content published", "version", release.Version, "by", release.PublishedBy, "items", release.Items)
	respondJSON(w, http.StatusOK, release)
}

func (h *AdminHandler) Counts(w http.ResponseWriter, r *http.Request) {
	counts, err := h.content.Counts(r.Context())
	if err != nil {
		h.fail(w, "counts", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"counts": counts})
}

func (h *AdminHandler) Releases(w http.ResponseWriter, r *http.Request) {
	list, err := h.content.Releases(r.Context())
	if err != nil {
		h.fail(w, "releases", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"releases": list})
}
