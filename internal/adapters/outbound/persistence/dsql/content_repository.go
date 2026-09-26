package dsql

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type contentRepository struct{ pool *pgxpool.Pool }

func NewContentRepository(pool *pgxpool.Pool) outbound.ContentRepository {
	return &contentRepository{pool: pool}
}

const itemColumns = `collection, lang, item_id, position, data, updated_at, updated_by`

func scanItem(row pgx.Row) (domain.ContentItem, error) {
	var it domain.ContentItem
	var data string
	err := row.Scan(&it.Collection, &it.Lang, &it.ID, &it.Position, &data, &it.UpdatedAt, &it.UpdatedBy)
	it.Data = []byte(data)
	return it, err
}

func (r *contentRepository) List(ctx context.Context, collection, lang string) ([]domain.ContentItem, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+itemColumns+` FROM content_items
		WHERE collection = $1 AND lang = $2 ORDER BY position, item_id`, collection, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.ContentItem{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *contentRepository) Get(ctx context.Context, collection, lang, id string) (domain.ContentItem, bool, error) {
	it, err := scanItem(r.pool.QueryRow(ctx, `SELECT `+itemColumns+` FROM content_items
		WHERE collection = $1 AND lang = $2 AND item_id = $3`, collection, lang, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ContentItem{}, false, nil
	}
	return it, err == nil, err
}

func (r *contentRepository) NextPosition(ctx context.Context, collection, lang string) (int, error) {
	var next int
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(position) + 1, 0) FROM content_items
		WHERE collection = $1 AND lang = $2`, collection, lang).Scan(&next)
	return next, err
}

func (r *contentRepository) Upsert(ctx context.Context, it domain.ContentItem) error {
	return retry(func() error {
		_, err := r.pool.Exec(ctx, `
			INSERT INTO content_items (`+itemColumns+`) VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (collection, lang, item_id) DO UPDATE SET
				position = EXCLUDED.position, data = EXCLUDED.data,
				updated_at = EXCLUDED.updated_at, updated_by = EXCLUDED.updated_by`,
			it.Collection, it.Lang, it.ID, it.Position, string(it.Data), it.UpdatedAt, it.UpdatedBy)
		return err
	})
}

func (r *contentRepository) Delete(ctx context.Context, collection, lang, id string) (bool, error) {
	var deleted int64
	err := retry(func() error {
		tag, err := r.pool.Exec(ctx, `DELETE FROM content_items WHERE collection = $1 AND lang = $2 AND item_id = $3`,
			collection, lang, id)
		deleted = tag.RowsAffected()
		return err
	})
	return deleted > 0, err
}

func (r *contentRepository) Counts(ctx context.Context) (map[string]map[string]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT collection, lang, count(*) FROM content_items GROUP BY collection, lang`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]int{}
	for rows.Next() {
		var c, l string
		var n int
		if err := rows.Scan(&c, &l, &n); err != nil {
			return nil, err
		}
		if out[c] == nil {
			out[c] = map[string]int{}
		}
		out[c][l] = n
	}
	return out, rows.Err()
}

func (r *contentRepository) LatestRelease(ctx context.Context) (domain.Release, bool, error) {
	var rel domain.Release
	err := r.pool.QueryRow(ctx, `SELECT version, published_at, published_by, items FROM content_releases
		ORDER BY version DESC LIMIT 1`).Scan(&rel.Version, &rel.PublishedAt, &rel.PublishedBy, &rel.Items)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Release{}, false, nil
	}
	return rel, err == nil, err
}

func (r *contentRepository) SaveRelease(ctx context.Context, rel domain.Release) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO content_releases (version, published_at, published_by, items)
		VALUES ($1, $2, $3, $4)`, rel.Version, rel.PublishedAt, rel.PublishedBy, rel.Items)
	return err
}

func (r *contentRepository) Releases(ctx context.Context, limit int) ([]domain.Release, error) {
	rows, err := r.pool.Query(ctx, `SELECT version, published_at, published_by, items FROM content_releases
		ORDER BY version DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []domain.Release{}
	for rows.Next() {
		var rel domain.Release
		if err := rows.Scan(&rel.Version, &rel.PublishedAt, &rel.PublishedBy, &rel.Items); err != nil {
			return nil, err
		}
		list = append(list, rel)
	}
	return list, rows.Err()
}
