package inbound

import (
	"context"
	"encoding/json"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

// ContentService is what the admin page does with the app's content.
type ContentService interface {
	Collections() []domain.Collection
	List(ctx context.Context, collection, lang string) ([]domain.ContentItem, error)
	// Save creates or replaces one item. position < 0 keeps the current one
	// (or appends, for a new item).
	Save(ctx context.Context, by domain.Admin, collection, lang, id string, data json.RawMessage, position int) (domain.ContentItem, error)
	Delete(ctx context.Context, by domain.Admin, collection, lang, id string) error
	// Publish freezes every collection and language into a new release the
	// app will download.
	Publish(ctx context.Context, by domain.Admin) (domain.Release, error)
	Releases(ctx context.Context) ([]domain.Release, error)
}
