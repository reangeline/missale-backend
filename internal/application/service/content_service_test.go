package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type memRepo struct {
	items    map[string]domain.ContentItem
	releases []domain.Release
}

func newMemRepo() *memRepo { return &memRepo{items: map[string]domain.ContentItem{}} }

func key(c, l, id string) string { return c + "/" + l + "/" + id }

func (m *memRepo) List(_ context.Context, c, l string) ([]domain.ContentItem, error) {
	var out []domain.ContentItem
	for _, it := range m.items {
		if it.Collection == c && it.Lang == l {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}
func (m *memRepo) Get(_ context.Context, c, l, id string) (domain.ContentItem, bool, error) {
	it, ok := m.items[key(c, l, id)]
	return it, ok, nil
}
func (m *memRepo) NextPosition(ctx context.Context, c, l string) (int, error) {
	list, _ := m.List(ctx, c, l)
	if len(list) == 0 {
		return 0, nil
	}
	return list[len(list)-1].Position + 1, nil
}
func (m *memRepo) Upsert(_ context.Context, it domain.ContentItem) error {
	m.items[key(it.Collection, it.Lang, it.ID)] = it
	return nil
}
func (m *memRepo) Delete(_ context.Context, c, l, id string) (bool, error) {
	_, ok := m.items[key(c, l, id)]
	delete(m.items, key(c, l, id))
	return ok, nil
}
func (m *memRepo) LatestRelease(context.Context) (domain.Release, bool, error) {
	if len(m.releases) == 0 {
		return domain.Release{}, false, nil
	}
	return m.releases[len(m.releases)-1], true, nil
}
func (m *memRepo) SaveRelease(_ context.Context, r domain.Release) error {
	m.releases = append(m.releases, r)
	return nil
}
func (m *memRepo) Releases(context.Context, int) ([]domain.Release, error) { return m.releases, nil }

type memPublisher struct {
	order []string
	files map[string][]byte
	imm   map[string]bool
}

func (p *memPublisher) Put(_ context.Context, path string, body []byte, immutable bool) error {
	if p.files == nil {
		p.files, p.imm = map[string][]byte{}, map[string]bool{}
	}
	p.order = append(p.order, path)
	p.files[path], p.imm[path] = body, immutable
	return nil
}

var admin = domain.Admin{Email: "admin@example.com"}

func word(id string) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"id": id, "quote": "Q " + id, "reference": "Mt 5, 3",
		"translationNote": "Figueiredo 1896", "context": "C"})
	return b
}

func TestSaveValidatesAgainstTheCollection(t *testing.T) {
	s := NewContentService(newMemRepo(), &memPublisher{})
	ctx := context.Background()
	cases := map[string]struct {
		collection, lang, id string
		data                 string
		want                 error
	}{
		"unknown collection": {"hymns", "pt", "a", string(word("a")), domain.ErrUnknownCollection},
		"unknown language":   {"word_of_day", "fr", "a", string(word("a")), domain.ErrUnknownLanguage},
		"bad id":             {"word_of_day", "pt", "Mateus 5", string(word("Mateus 5")), domain.ErrInvalidContent},
		"id mismatch":        {"word_of_day", "pt", "a", string(word("b")), domain.ErrInvalidContent},
		"missing field":      {"word_of_day", "pt", "a", `{"id":"a","quote":"q","reference":"r","translationNote":"t"}`, domain.ErrInvalidContent},
		"unknown field":      {"word_of_day", "pt", "a", strings.Replace(string(word("a")), `"context"`, `"extra":"x","context"`, 1), domain.ErrInvalidContent},
		"not text":           {"word_of_day", "pt", "a", `{"id":"a","quote":3,"reference":"r","translationNote":"t","context":"c"}`, domain.ErrInvalidContent},
		"not an object":      {"word_of_day", "pt", "a", `["a"]`, domain.ErrInvalidContent},
	}
	for name, c := range cases {
		_, err := s.Save(ctx, admin, c.collection, c.lang, c.id, json.RawMessage(c.data), -1)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
}

func TestSaveAppendsNewItemsAndKeepsPositionOnEdit(t *testing.T) {
	repo := newMemRepo()
	s := NewContentService(repo, &memPublisher{})
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		if _, err := s.Save(ctx, admin, "word_of_day", "pt", id, word(id), -1); err != nil {
			t.Fatal(err)
		}
	}
	edited, err := s.Save(ctx, admin, "word_of_day", "pt", "a", word("a"), -1)
	if err != nil || edited.Position != 0 || edited.UpdatedBy != "admin@example.com" {
		t.Fatalf("edit kept position %d by %q: %v", edited.Position, edited.UpdatedBy, err)
	}
	if c := repo.items[key("word_of_day", "pt", "c")]; c.Position != 2 {
		t.Fatalf("third item at %d", c.Position)
	}
}

func TestPublishWritesFilesThenTheManifest(t *testing.T) {
	repo, pub := newMemRepo(), &memPublisher{}
	s := NewContentService(repo, pub).(*contentService)
	s.now = func() time.Time { return time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	for _, id := range []string{"b", "a"} {
		if _, err := s.Save(ctx, admin, "word_of_day", "pt", id, word(id), -1); err != nil {
			t.Fatal(err)
		}
	}

	release, err := s.Publish(ctx, admin)
	if err != nil || release.Version != 1 || release.Items != 2 {
		t.Fatalf("release %+v: %v", release, err)
	}
	if last := pub.order[len(pub.order)-1]; last != "manifest.json" || pub.imm["manifest.json"] {
		t.Fatalf("manifest must go last and not be immutable: %v", pub.order)
	}
	var m domain.Manifest
	if err := json.Unmarshal(pub.files["manifest.json"], &m); err != nil {
		t.Fatal(err)
	}
	info, ok := m.Files["word_of_day"]["pt"]
	if !ok || info.Path != "v1/word_of_day/pt.json" || info.Count != 2 || !pub.imm[info.Path] {
		t.Fatalf("manifest entry %+v", info)
	}
	if _, ok := m.Files["word_of_day"]["en"]; ok {
		t.Fatal("an empty language must not be published (the app keeps its bundled list)")
	}
	sum := sha256.Sum256(pub.files[info.Path])
	if hex.EncodeToString(sum[:]) != info.SHA256 {
		t.Fatal("manifest hash does not match the file")
	}
	var list []map[string]string
	if err := json.Unmarshal(pub.files[info.Path], &list); err != nil || len(list) != 2 || list[0]["id"] != "b" {
		t.Fatalf("published list keeps the admin's order: %v %v", list, err)
	}

	again, _ := s.Publish(ctx, admin)
	if again.Version != 2 {
		t.Fatalf("second release is version %d", again.Version)
	}
}

func TestDeleteUnknownItemIsNotFound(t *testing.T) {
	s := NewContentService(newMemRepo(), &memPublisher{})
	if err := s.Delete(context.Background(), admin, "word_of_day", "pt", "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func saint(mutate func(map[string]any)) json.RawMessage {
	d := map[string]any{
		"id": "agostinho", "dateKey": "08-28", "name": "Santo Agostinho", "role": "Bispo",
		"rank": "Memória", "calendarNote": "Calendário Romano Geral · 28 de agosto",
		"bioParagraphs":     []any{"Nasceu em Tagaste.", "Morreu em Hipona."},
		"whyItMattersToday": "Fala a quem procura.", "prayer": "Santo Agostinho, rogai por nós.",
	}
	if mutate != nil {
		mutate(d)
	}
	b, _ := json.Marshal(d)
	return b
}

func TestSaintsValidation(t *testing.T) {
	s := NewContentService(newMemRepo(), &memPublisher{})
	ctx := context.Background()
	bad := map[string]func(map[string]any){
		"date not MM-dd":        func(d map[string]any) { d["dateKey"] = "28/08" },
		"impossible month":      func(d map[string]any) { d["dateKey"] = "13-01" },
		"no biography":          func(d map[string]any) { d["bioParagraphs"] = []any{} },
		"only empty paragraphs": func(d map[string]any) { d["bioParagraphs"] = []any{"  ", ""} },
		"paragraphs not a list": func(d map[string]any) { d["bioParagraphs"] = "texto" },
		"story without source": func(d map[string]any) {
			d["stories"] = []any{map[string]any{"title": "O lobo", "body": "..."}}
		},
		"story with extra field": func(d map[string]any) {
			d["stories"] = []any{map[string]any{"title": "t", "body": "b", "source": "s", "x": "y"}}
		},
	}
	for name, mutate := range bad {
		if _, err := s.Save(ctx, admin, "saints", "pt", "agostinho", saint(mutate), -1); !errors.Is(err, domain.ErrInvalidContent) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestSaintsAlwaysCarryEveryField(t *testing.T) {
	s := NewContentService(newMemRepo(), &memPublisher{})
	item, err := s.Save(context.Background(), admin, "saints", "pt", "agostinho",
		saint(func(d map[string]any) { d["bioParagraphs"] = []any{"Nasceu em Tagaste.", "   "} }), -1)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(item.Data, &got)
	// Optional fields left out still come back, empty, for the app's decoder.
	if got["lifespan"] != "" || got["artworkName"] != "" {
		t.Fatalf("optional text fields: %v / %v", got["lifespan"], got["artworkName"])
	}
	if stories, ok := got["stories"].([]any); !ok || len(stories) != 0 {
		t.Fatalf("stories should be an empty list, got %#v", got["stories"])
	}
	if bio := got["bioParagraphs"].([]any); len(bio) != 1 {
		t.Fatalf("empty paragraph should be dropped: %v", bio)
	}

	withStory, err := s.Save(context.Background(), admin, "saints", "pt", "agostinho", saint(func(d map[string]any) {
		d["stories"] = []any{map[string]any{"title": "Tolle, lege", "body": "No jardim de Milão…", "source": "Confissões VIII"}}
	}), -1)
	if err != nil || !strings.Contains(string(withStory.Data), "Confissões VIII") {
		t.Fatalf("story not kept: %s %v", withStory.Data, err)
	}
}

func TestMoodReliefsOnlyAcceptKnownStates(t *testing.T) {
	s := NewContentService(newMemRepo(), &memPublisher{})
	reply := func(state string) json.RawMessage {
		b, _ := json.Marshal(map[string]string{"id": "grief-01", "stateID": state, "title": "As lágrimas, meu pão",
			"psalmRef": "Salmo 42, 4", "psalmText": "…", "psalmWhy": "…", "saintName": "Santa Mônica",
			"saintWhy": "…", "stepTitle": "Um passo concreto", "stepBody": "…"})
		return b
	}
	if _, err := s.Save(context.Background(), admin, "mood_reliefs", "pt", "grief-01", reply("grief"), -1); err != nil {
		t.Fatalf("known state refused: %v", err)
	}
	for _, bad := range []string{"sad", "Grief", "grief ", "peace|grief"} {
		if _, err := s.Save(context.Background(), admin, "mood_reliefs", "pt", "grief-01", reply(bad), -1); !errors.Is(err, domain.ErrInvalidContent) && bad != "grief " {
			t.Errorf("state %q accepted: %v", bad, err)
		}
	}
}
