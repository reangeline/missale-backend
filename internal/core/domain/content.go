package domain

import (
	"encoding/json"
	"time"
)

// Languages the app ships. Each language has its own list per collection:
// liturgical texts have their own approved wording in each language, so the
// lists are authored side by side, not translated one from another.
var ContentLanguages = []string{"pt", "en", "es"}

// FieldType tells the admin page which input to draw.
type FieldType string

const (
	FieldText     FieldType = "text"     // one line
	FieldLongText FieldType = "longtext" // paragraphs
)

type Field struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Help     string    `json:"help,omitempty"`
}

// Collection is one kind of content the app reads (the word of the day, the
// saints…). Fields mirror the app's Codable struct: the published JSON must
// decode straight into it.
type Collection struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Fields      []Field `json:"fields"`
}

// Collections is the registry of what the admin page edits. A collection
// added here also needs the app to read it (see holy_messages RemoteContent).
var Collections = []Collection{
	{
		Key:         "word_of_day",
		Label:       "Palavra do dia",
		Description: "O versículo do dia na tela Hoje e no widget. Cada idioma tem a sua lista.",
		Fields: []Field{
			{Key: "id", Label: "Identificador", Type: FieldText, Required: true, Help: "Único e fixo, ex.: mateus-5-3-pt. Não mude depois de publicado."},
			{Key: "quote", Label: "Texto", Type: FieldLongText, Required: true},
			{Key: "reference", Label: "Referência", Type: FieldText, Required: true, Help: "Ex.: Mateus 5, 3"},
			{Key: "translationNote", Label: "Edição da tradução", Type: FieldText, Required: true, Help: "Ex.: Figueiredo 1896 · domínio público, grafia atualizada"},
			{Key: "context", Label: "Contexto", Type: FieldLongText, Required: true},
		},
	},
}

func CollectionByKey(key string) (Collection, bool) {
	for _, c := range Collections {
		if c.Key == key {
			return c, true
		}
	}
	return Collection{}, false
}

func IsContentLanguage(lang string) bool {
	for _, l := range ContentLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

// ContentItem is one entry of a collection in one language. Data holds the
// fields as a JSON object (the app's struct, encoded).
type ContentItem struct {
	Collection string          `json:"collection"`
	Lang       string          `json:"lang"`
	ID         string          `json:"id"`
	Position   int             `json:"position"`
	Data       json.RawMessage `json:"data"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	UpdatedBy  string          `json:"updatedBy"`
}

// Release is one publication: every collection and language, frozen into
// files under a version the app can compare with what it has.
type Release struct {
	Version     int       `json:"version"`
	PublishedAt time.Time `json:"publishedAt"`
	PublishedBy string    `json:"publishedBy"`
	Items       int       `json:"items"`
}

// Manifest is what the app downloads first. Files maps collection → lang →
// published file; the app fetches only files whose hash changed.
type Manifest struct {
	Version     int                                    `json:"version"`
	PublishedAt time.Time                              `json:"publishedAt"`
	Files       map[string]map[string]ManifestFileInfo `json:"files"`
}

type ManifestFileInfo struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Count  int    `json:"count"`
}

// Admin is who is signed in to the admin page.
type Admin struct {
	Username string
	Email    string
}

// AdminSession is the admin page's session, or a challenge to set a new
// password (first sign-in with the temporary password Cognito emailed).
type AdminSession struct {
	AccessToken       string `json:"accessToken,omitempty"`
	RefreshToken      string `json:"refreshToken,omitempty"`
	ExpiresIn         int32  `json:"expiresIn,omitempty"`
	NewPasswordNeeded bool   `json:"newPasswordNeeded,omitempty"`
	Session           string `json:"session,omitempty"`
}
