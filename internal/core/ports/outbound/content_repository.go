package outbound

import (
	"context"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type ContentRepository interface {
	// List returns one collection in one language, ordered by position.
	List(ctx context.Context, collection, lang string) ([]domain.ContentItem, error)
	Get(ctx context.Context, collection, lang, id string) (domain.ContentItem, bool, error)
	// NextPosition is one past the highest position in the list.
	NextPosition(ctx context.Context, collection, lang string) (int, error)
	Upsert(ctx context.Context, item domain.ContentItem) error
	Delete(ctx context.Context, collection, lang, id string) (bool, error)
	// Counts returns how many items each collection has per language.
	Counts(ctx context.Context) (map[string]map[string]int, error)

	LatestRelease(ctx context.Context) (domain.Release, bool, error)
	SaveRelease(ctx context.Context, r domain.Release) error
	Releases(ctx context.Context, limit int) ([]domain.Release, error)
}
