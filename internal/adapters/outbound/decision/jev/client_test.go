package jev

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

func TestDecideRetriesServerErrorsAndSendsFixedModel(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		if body["model"] != "typesafe/jev-1.13" {
			t.Errorf("model = %v", body["model"])
		}
		if calls == 1 {
			w.WriteHeader(520) // seen from OpenRouter under concurrency
			return
		}
		_, _ = w.Write([]byte(`{"answers":{"risk":{"type":"noul","noul":0.9}},"usage":{}}`))
	}))
	defer srv.Close()

	c := newClient("k", "typesafe/jev-1.13")
	c.url = srv.URL
	answers, err := c.Decide(context.Background(), "texto", map[string]domain.Question{"risk": {Type: "noul", Instructions: "q"}})
	if err != nil || calls != 2 || string(answers) != `{"risk":{"type":"noul","noul":0.9}}` {
		t.Fatalf("answers=%s err=%v calls=%d", answers, err, calls)
	}
}

func TestDecideDoesNotRetryClientErrors(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(400)
	}))
	defer srv.Close()
	c := newClient("k", "m")
	c.url = srv.URL
	if _, err := c.Decide(context.Background(), "x", nil); err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
