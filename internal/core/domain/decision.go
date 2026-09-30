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
	// Free says whether a call without an active subscription may come out
	// of the account's free allowance. Only the onboarding's orientação sends
	// true; the other features send false and get ErrNotSubscribed without
	// spending anything. Absent (nil), for app builds that predate the field,
	// it behaves as true.
	Free *bool `json:"free,omitempty"`
}

// MaySpendFree tells whether the request allows spending the free allowance:
// true when Free is absent (the old contract) or true.
func (r DecisionRequest) MaySpendFree() bool {
	return r.Free == nil || *r.Free
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
