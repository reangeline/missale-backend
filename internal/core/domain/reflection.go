package domain

// Passage is the Bible passage Jev chose for the person.
type Passage struct {
	Reference string `json:"reference"`
	Text      string `json:"text"`
}

// Saint is the saint Jev chose, with the summary of their life from the
// app's content.
type Saint struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

// ReflectionRequest asks for a short reflection, in the voice of a priest,
// connecting what the person wrote (State) to the passage and the saint
// already chosen. State and the reflection are never stored or logged.
type ReflectionRequest struct {
	State    string  `json:"state"`
	Passage  Passage `json:"passage"`
	Saint    Saint   `json:"saint"`
	Language string  `json:"language"`
	// Context is optional: the onboarding questionnaire answers, already in
	// the app's language, one "Question: answer" per line. Never logged.
	Context string `json:"context,omitempty"`
	// Crisis is set when the app detected risk of suicide or self-harm: the
	// reflection then speaks of God but clearly points to help, and Passage
	// and Saint become optional.
	Crisis bool `json:"crisis,omitempty"`
	// Free has the same meaning as DecisionRequest.Free.
	Free *bool `json:"free,omitempty"`
}

// MaySpendFree: true when Free is absent or true.
func (r ReflectionRequest) MaySpendFree() bool {
	return r.Free == nil || *r.Free
}

const (
	MaxPassageChars = 4000
	MaxSummaryChars = 2000
	// MaxContextChars bounds ReflectionRequest.Context.
	MaxContextChars = 1500
	// MaxLabelChars bounds Passage.Reference and Saint.Name.
	MaxLabelChars = 200
)

// ReflectionLanguages are the languages the reflection may be written in.
var ReflectionLanguages = map[string]bool{"pt": true, "en": true, "es": true}
