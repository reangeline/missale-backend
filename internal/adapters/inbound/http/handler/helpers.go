package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

const (
	maxBodyBytes      = 64 << 10
	maxAdminBodyBytes = 512 << 10 // a content item with long texts
)

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONLimit(w, r, v, maxBodyBytes)
}

func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	return dec.Decode(v) == nil
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// RespondError writes {"error": code}. Exported for the middleware.
func RespondError(w http.ResponseWriter, status int, code string) {
	respondJSON(w, status, map[string]string{"error": code})
}

// respondDomainError maps domain errors to HTTP. Anything unexpected is a 500
// with a generic code; the detail goes to the log, never to the client.
func respondDomainError(w http.ResponseWriter, err error) (status int) {
	for _, m := range []struct {
		err    error
		status int
	}{
		{domain.ErrUnauthorized, http.StatusUnauthorized},
		{domain.ErrInvalidAppleToken, http.StatusUnauthorized},
		{domain.ErrSessionExpired, http.StatusUnauthorized},
		{domain.ErrNotSubscribed, http.StatusPaymentRequired},
		{domain.ErrDailyLimit, http.StatusTooManyRequests},
		{domain.ErrInvalidState, http.StatusBadRequest},
		{domain.ErrInvalidQuestions, http.StatusBadRequest},
		{domain.ErrDecisionEngine, http.StatusBadGateway},
		{domain.ErrForbidden, http.StatusForbidden},
		{domain.ErrInvalidCredentials, http.StatusUnauthorized},
		{domain.ErrUnknownCollection, http.StatusNotFound},
		{domain.ErrUnknownLanguage, http.StatusNotFound},
		{domain.ErrNotFound, http.StatusNotFound},
	} {
		if errors.Is(err, m.err) {
			RespondError(w, m.status, m.err.Error())
			return m.status
		}
	}
	// The admin page shows why an item was refused ("quote" is required…).
	if errors.Is(err, domain.ErrInvalidContent) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": domain.ErrInvalidContent.Error(), "detail": err.Error()})
		return http.StatusBadRequest
	}
	RespondError(w, http.StatusInternalServerError, "internal_error")
	return http.StatusInternalServerError
}
