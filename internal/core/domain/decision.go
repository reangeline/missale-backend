package domain

import "encoding/json"

// Question is one typed question for Jev: "noul" (yes/no probability),
// "choice" (Criteria is an object {key: description}) or "score"
// (Criteria is an array of levels, lowest first).
type Question struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria,omitempty"`
}

// DecisionRequest is what the app asks about what the user wrote (State).
// State is never stored or logged anywhere on the server.
type DecisionRequest struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

// Limits on what the app may forward, so the proxy can't be used as an
// unbounded, general-purpose Jev endpoint.
const (
	MaxStateChars        = 2000
	MaxQuestions         = 3
	MaxInstructionsChars = 300
	MaxCriteria          = 32
	MaxCriterionChars    = 400
)
