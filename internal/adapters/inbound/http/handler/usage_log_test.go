package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLogUseRecordsEveryOutcomeWithoutTheText(t *testing.T) {
	cases := []struct {
		status int
		err    error
		level  string
	}{
		{200, nil, "INFO"},
		{400, nil, "INFO"},
		{429, errors.New("daily limit"), "INFO"},
		{402, errors.New("not subscribed"), "WARN"},
		{502, errors.New("engine"), "WARN"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		r := httptest.NewRequest("POST", "/v1/decisions", strings.NewReader(`{"state":"segredo do leitor"}`))
		r.Header.Set("X-Subscription", "jws")
		logUse(slog.New(slog.NewJSONHandler(&buf, nil)), "decision", r, "u1", c.status, time.Now(), c.err)
		out := buf.String()
		if !strings.Contains(out, `"level":"`+c.level+`"`) || !strings.Contains(out, `"subscription":true`) {
			t.Errorf("status %d: unexpected line %s", c.status, out)
		}
		if strings.Contains(out, "segredo") {
			t.Errorf("status %d: the reader's text reached the log", c.status)
		}
	}
}
