package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type contentService struct {
	repo      outbound.ContentRepository
	publisher outbound.ContentPublisher
	now       func() time.Time
}

func NewContentService(repo outbound.ContentRepository, publisher outbound.ContentPublisher) inbound.ContentService {
	return &contentService{repo: repo, publisher: publisher, now: time.Now}
}

func (s *contentService) Collections() []domain.Collection { return domain.Collections }

func (s *contentService) List(ctx context.Context, collection, lang string) ([]domain.ContentItem, error) {
	if _, err := resolve(collection, lang); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, collection, lang)
}

var itemIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,99}$`)

const maxFieldChars = 20000

func (s *contentService) Save(ctx context.Context, by domain.Admin, collection, lang, id string, data json.RawMessage, position int) (domain.ContentItem, error) {
	c, err := resolve(collection, lang)
	if err != nil {
		return domain.ContentItem{}, err
	}
	normalized, err := validateItem(c, id, data)
	if err != nil {
		return domain.ContentItem{}, err
	}
	if position < 0 {
		existing, found, err := s.repo.Get(ctx, collection, lang, id)
		if err != nil {
			return domain.ContentItem{}, err
		}
		if found {
			position = existing.Position
		} else if position, err = s.repo.NextPosition(ctx, collection, lang); err != nil {
			return domain.ContentItem{}, err
		}
	}
	item := domain.ContentItem{
		Collection: collection, Lang: lang, ID: id, Position: position,
		Data: normalized, UpdatedAt: s.now().UTC(), UpdatedBy: by.Email,
	}
	if err := s.repo.Upsert(ctx, item); err != nil {
		return domain.ContentItem{}, err
	}
	return item, nil
}

func (s *contentService) Delete(ctx context.Context, _ domain.Admin, collection, lang, id string) error {
	if _, err := resolve(collection, lang); err != nil {
		return err
	}
	found, err := s.repo.Delete(ctx, collection, lang, id)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrNotFound
	}
	return nil
}

// Publish writes every collection/language as an immutable file under a new
// version, then the manifest that points at them. The manifest goes last, so
// an app that reads it always finds every file it names.
func (s *contentService) Publish(ctx context.Context, by domain.Admin) (domain.Release, error) {
	latest, _, err := s.repo.LatestRelease(ctx)
	if err != nil {
		return domain.Release{}, err
	}
	release := domain.Release{Version: latest.Version + 1, PublishedAt: s.now().UTC(), PublishedBy: by.Email}
	manifest := domain.Manifest{Version: release.Version, PublishedAt: release.PublishedAt,
		Files: map[string]map[string]domain.ManifestFileInfo{}}

	for _, c := range domain.Collections {
		manifest.Files[c.Key] = map[string]domain.ManifestFileInfo{}
		for _, lang := range domain.ContentLanguages {
			items, err := s.repo.List(ctx, c.Key, lang)
			if err != nil {
				return domain.Release{}, err
			}
			if len(items) == 0 {
				continue // the app keeps its bundled list for this language
			}
			body, err := publishedList(items)
			if err != nil {
				return domain.Release{}, err
			}
			sum := sha256.Sum256(body)
			path := fmt.Sprintf("v%d/%s/%s.json", release.Version, c.Key, lang)
			if err := s.publisher.Put(ctx, path, body, true); err != nil {
				return domain.Release{}, fmt.Errorf("publish %s: %w", path, err)
			}
			manifest.Files[c.Key][lang] = domain.ManifestFileInfo{Path: path, SHA256: hex.EncodeToString(sum[:]), Count: len(items)}
			release.Items += len(items)
		}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return domain.Release{}, err
	}
	if err := s.publisher.Put(ctx, "manifest.json", body, false); err != nil {
		return domain.Release{}, fmt.Errorf("publish manifest: %w", err)
	}
	if err := s.repo.SaveRelease(ctx, release); err != nil {
		return domain.Release{}, err
	}
	return release, nil
}

func (s *contentService) Counts(ctx context.Context) (map[string]map[string]int, error) {
	counts, err := s.repo.Counts(ctx)
	if err != nil {
		return nil, err
	}
	// Every collection and language present, zero when empty.
	out := map[string]map[string]int{}
	for _, c := range domain.Collections {
		out[c.Key] = map[string]int{}
		for _, l := range domain.ContentLanguages {
			out[c.Key][l] = counts[c.Key][l]
		}
	}
	return out, nil
}

func (s *contentService) Releases(ctx context.Context) ([]domain.Release, error) {
	return s.repo.Releases(ctx, 20)
}

func resolve(collection, lang string) (domain.Collection, error) {
	c, ok := domain.CollectionByKey(collection)
	if !ok {
		return domain.Collection{}, domain.ErrUnknownCollection
	}
	if !domain.IsContentLanguage(lang) {
		return domain.Collection{}, domain.ErrUnknownLanguage
	}
	return c, nil
}

// validateItem accepts exactly the collection's fields with the right shapes,
// the required ones filled and "id" matching the path. The result always
// carries every declared field ("" or [] when empty), so the app can decode
// it into a struct with non-optional properties.
func validateItem(c domain.Collection, id string, data json.RawMessage) (json.RawMessage, error) {
	if !itemIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: id must be lowercase letters, digits and hyphens", domain.ErrInvalidContent)
	}
	var fields map[string]any
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%w: data must be a JSON object", domain.ErrInvalidContent)
	}
	clean, err := cleanFields(c.Fields, fields, "")
	if err != nil {
		return nil, err
	}
	if got, ok := clean["id"].(string); ok && got != id {
		return nil, fmt.Errorf("%w: data.id %q differs from %q", domain.ErrInvalidContent, got, id)
	}
	return json.Marshal(clean)
}

func cleanFields(defs []domain.Field, fields map[string]any, prefix string) (map[string]any, error) {
	known := map[string]domain.Field{}
	for _, f := range defs {
		known[f.Key] = f
	}
	for key := range fields {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("%w: unknown field %q", domain.ErrInvalidContent, prefix+key)
		}
	}
	clean := map[string]any{}
	for _, f := range defs {
		name := prefix + f.Key
		value, present := fields[f.Key]
		switch f.Type {
		case domain.FieldText, domain.FieldLongText:
			text := ""
			if present && value != nil {
				s, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("%w: %q must be text", domain.ErrInvalidContent, name)
				}
				text = s
			}
			if f.Type == domain.FieldText {
				text = strings.TrimSpace(text)
			}
			if err := checkText(f, name, text); err != nil {
				return nil, err
			}
			clean[f.Key] = text
		case domain.FieldParagraphs:
			list, err := asList(value, present, name)
			if err != nil {
				return nil, err
			}
			paragraphs := []string{}
			for i, item := range list {
				s, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("%w: %s[%d] must be text", domain.ErrInvalidContent, name, i)
				}
				if s = strings.TrimSpace(s); s == "" {
					continue // an emptied paragraph box
				}
				if err := checkText(domain.Field{Key: f.Key}, name, s); err != nil {
					return nil, err
				}
				paragraphs = append(paragraphs, s)
			}
			if f.Required && len(paragraphs) == 0 {
				return nil, fmt.Errorf("%w: %q needs at least one paragraph", domain.ErrInvalidContent, name)
			}
			clean[f.Key] = paragraphs
		case domain.FieldItems:
			list, err := asList(value, present, name)
			if err != nil {
				return nil, err
			}
			records := []map[string]any{}
			for i, item := range list {
				obj, ok := item.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("%w: %s[%d] must be an object", domain.ErrInvalidContent, name, i)
				}
				rec, err := cleanFields(f.Subfields, obj, fmt.Sprintf("%s[%d].", name, i))
				if err != nil {
					return nil, err
				}
				records = append(records, rec)
			}
			if f.Required && len(records) == 0 {
				return nil, fmt.Errorf("%w: %q needs at least one entry", domain.ErrInvalidContent, name)
			}
			clean[f.Key] = records
		default:
			return nil, fmt.Errorf("%w: field %q has unknown type", domain.ErrInvalidContent, name)
		}
	}
	return clean, nil
}

func asList(value any, present bool, name string) ([]any, error) {
	if !present || value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %q must be a list", domain.ErrInvalidContent, name)
	}
	return list, nil
}

func checkText(f domain.Field, name, text string) error {
	if f.Required && strings.TrimSpace(text) == "" {
		return fmt.Errorf("%w: %q is required", domain.ErrInvalidContent, name)
	}
	if utf8.RuneCountInString(text) > maxFieldChars {
		return fmt.Errorf("%w: %q is too long", domain.ErrInvalidContent, name)
	}
	if f.Pattern != "" && text != "" && !regexp.MustCompile(f.Pattern).MatchString(text) {
		return fmt.Errorf("%w: %q has the wrong format", domain.ErrInvalidContent, name)
	}
	return nil
}

// publishedList is the file the app decodes: an array of the items' objects.
func publishedList(items []domain.ContentItem) ([]byte, error) {
	list := make([]json.RawMessage, len(items))
	for i, item := range items {
		list[i] = item.Data
	}
	return json.Marshal(list)
}
