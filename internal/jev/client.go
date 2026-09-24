// Package jev forwards typed questions to TypeSafe's Jev through OpenRouter's
// System One endpoint. Jev returns decisions (choice / score / noul), never text.
package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const endpoint = "https://openrouter.ai/api/v1/systemone"

// Question mirrors the OpenRouter request shape. Criteria is an object
// {key: description} for choice and an array of levels for score.
type Question struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

type Client struct {
	apiKey string
	model  string
	http   *http.Client
	url    string
}

func NewClient(apiKey, model string) *Client {
	return &Client{apiKey: apiKey, model: model, http: &http.Client{Timeout: 15 * time.Second}, url: endpoint}
}

// Decide sends one state and its questions; returns Jev's "answers" object as is.
func (c *Client) Decide(ctx context.Context, state string, questions map[string]Question) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"model": c.model, "state": state, "questions": questions})
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		answers, retryable, err := c.post(ctx, body)
		if err == nil {
			return answers, nil
		}
		lastErr = err
		if !retryable {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 400 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (c *Client) post(ctx context.Context, body []byte) (json.RawMessage, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("jev: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		// 5xx (seen: 520 under concurrency) and 429 are worth another try.
		retryable := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return nil, retryable, fmt.Errorf("jev: HTTP %d: %.300s", resp.StatusCode, raw)
	}
	var out struct {
		Answers json.RawMessage `json:"answers"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Answers) == 0 {
		return nil, false, fmt.Errorf("jev: unexpected response: %.300s", raw)
	}
	return out.Answers, false, nil
}
