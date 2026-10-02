package handler

import (
	"log/slog"
	"net/http"
	"time"
)

// logUse writes one line per decision or reflection request. Before it only
// 402s and 5xx were logged, so a success, a 400, a 401 or a 429 left no trace
// and "the analysis didn't work" could not be told apart from "it worked".
// What the reader wrote is never logged: only who, the outcome, whether a
// subscription was sent, and how long it took.
func logUse(log *slog.Logger, msg string, r *http.Request, user string, status int, start time.Time, err error) {
	attrs := []any{
		"user", user,
		"status", status,
		"subscription", r.Header.Get("X-Subscription") != "",
		"ms", time.Since(start).Milliseconds(),
	}
	switch {
	case status >= 500 || status == http.StatusPaymentRequired:
		log.Warn(msg, append(attrs, "err", err)...)
	case err != nil:
		log.Info(msg, append(attrs, "err", err)...)
	default:
		log.Info(msg, attrs...)
	}
}
