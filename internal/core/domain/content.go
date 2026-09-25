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
	FieldText       FieldType = "text"       // one line
	FieldLongText   FieldType = "longtext"   // a block of text
	FieldParagraphs FieldType = "paragraphs" // a list of paragraphs ([]string)
	FieldItems      FieldType = "items"      // a list of small records, each with Subfields
)

type Field struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	Help     string    `json:"help,omitempty"`
	// Pattern, when set, is a regular expression a text field must match.
	Pattern string `json:"pattern,omitempty"`
	// Subfields of each record in an "items" field (text or longtext only).
	Subfields []Field `json:"subfields,omitempty"`
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
	{
		Key:         "saints",
		Label:       "Santos",
		Description: "O santo do dia, o arquivo de santos e o calendário. Um santo por data; cada idioma tem a sua lista.",
		Fields: []Field{
			{Key: "id", Label: "Identificador", Type: FieldText, Required: true, Help: "Único e fixo, igual nos três idiomas (ex.: agostinho). Não mude depois de publicado."},
			{Key: "dateKey", Label: "Data da memória", Type: FieldText, Required: true, Pattern: `^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`, Help: "Mês-dia, ex.: 08-28 para 28 de agosto."},
			{Key: "name", Label: "Nome", Type: FieldText, Required: true},
			{Key: "lifespan", Label: "Período de vida", Type: FieldText, Help: "Ex.: 354–430 ou c. 480–547"},
			{Key: "role", Label: "Quem foi", Type: FieldText, Required: true, Help: "Ex.: Bispo de Hipona, teólogo e Doutor da Igreja"},
			{Key: "rank", Label: "Grau litúrgico", Type: FieldText, Required: true, Help: "Solenidade, Festa, Memória ou Memória facultativa (o app traduz esses quatro)."},
			{Key: "calendarNote", Label: "Nota do calendário", Type: FieldText, Required: true, Help: "Ex.: Calendário Romano Geral · 28 de agosto"},
			{Key: "bioParagraphs", Label: "Biografia", Type: FieldParagraphs, Required: true},
			{Key: "whyItMattersToday", Label: "Por que importa hoje", Type: FieldLongText, Required: true},
			{Key: "prayer", Label: "Oração", Type: FieldLongText, Required: true},
			{Key: "artworkName", Label: "Arte (nome da imagem no app)", Type: FieldText, Help: "Só imagens que já vêm no app. Deixe vazio se não houver arte."},
			{Key: "stories", Label: "Histórias e milagres", Type: FieldItems, Help: "Cada uma com a sua fonte.", Subfields: []Field{
				{Key: "title", Label: "Título", Type: FieldText, Required: true},
				{Key: "body", Label: "Texto", Type: FieldLongText, Required: true},
				{Key: "source", Label: "Fonte", Type: FieldText, Required: true},
			}},
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
