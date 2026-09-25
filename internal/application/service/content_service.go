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

// validateItem accepts exactly the collection's fields, as strings, with the
// required ones filled and "id" matching the path. Returns the object
// re-encoded, so what is stored is always well-formed.
func validateItem(c domain.Collection, id string, data json.RawMessage) (json.RawMessage, error) {
	if !itemIDPattern.MatchString(id) {
		return nil, fmt.Errorf("%w: id must be lowercase letters, digits and hyphens", domain.ErrInvalidContent)
	}
	var fields map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&fields); err != nil || fields == nil {
		return nil, fmt.Errorf("%w: data must be a JSON object", domain.ErrInvalidContent)
	}
	known := map[string]domain.Field{}
	for _, f := range c.Fields {
		known[f.Key] = f
	}
	clean := map[string]string{}
	for key, value := range fields {
		f, ok := known[key]
		if !ok {
			return nil, fmt.Errorf("%w: unknown field %q", domain.ErrInvalidContent, key)
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("%w: %q must be text", domain.ErrInvalidContent, key)
		}
		if f.Type == domain.FieldText {
			text = strings.TrimSpace(text)
		}
		if utf8.RuneCountInString(text) > maxFieldChars {
			return nil, fmt.Errorf("%w: %q is too long", domain.ErrInvalidContent, key)
		}
		clean[key] = text
	}
	for _, f := range c.Fields {
		if f.Required && strings.TrimSpace(clean[f.Key]) == "" {
			return nil, fmt.Errorf("%w: %q is required", domain.ErrInvalidContent, f.Key)
		}
	}
	if got, ok := clean["id"]; ok && got != id {
		return nil, fmt.Errorf("%w: data.id %q differs from %q", domain.ErrInvalidContent, got, id)
	}
	return json.Marshal(clean)
}

// publishedList is the file the app decodes: an array of the items' objects.
func publishedList(items []domain.ContentItem) ([]byte, error) {
	list := make([]json.RawMessage, len(items))
	for i, item := range items {
		list[i] = item.Data
	}
	return json.Marshal(list)
}
