package domain

import "errors"

var (
	ErrUnauthorized      = errors.New("unauthorized")
	ErrInvalidAppleToken = errors.New("invalid_apple_token")
	ErrSessionExpired    = errors.New("session_expired")
	ErrNotSubscribed     = errors.New("subscription_required")
	ErrDailyLimit        = errors.New("daily_limit")
	ErrInvalidState      = errors.New("invalid_state")
	ErrInvalidQuestions  = errors.New("invalid_questions")
	ErrDecisionEngine    = errors.New("jev_unavailable")

	ErrInvalidReflection    = errors.New("invalid_reflection")
	ErrReflectionEngine     = errors.New("reflection_unavailable")
	ErrReflectionNotEnabled = errors.New("reflection_not_configured")

	ErrForbidden          = errors.New("forbidden")
	ErrInvalidCredentials = errors.New("invalid_credentials")
	ErrUnknownCollection  = errors.New("unknown_collection")
	ErrUnknownLanguage    = errors.New("unknown_language")
	ErrInvalidContent     = errors.New("invalid_content")
	ErrNotFound           = errors.New("not_found")
)
