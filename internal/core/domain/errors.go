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
)
