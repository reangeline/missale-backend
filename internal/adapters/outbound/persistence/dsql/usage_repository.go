package dsql

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type usageRepository struct{ pool *pgxpool.Pool }

func NewUsageRepository(pool *pgxpool.Pool) outbound.UsageRepository {
	return &usageRepository{pool: pool}
}

func (r *usageRepository) Reserve(ctx context.Context, userID string) (int, error) {
	var calls int
	err := retry(func() error {
		return r.pool.QueryRow(ctx, `
			INSERT INTO decision_usage (user_id, day, calls) VALUES ($1, CURRENT_DATE, 1)
			ON CONFLICT (user_id, day) DO UPDATE SET calls = decision_usage.calls + 1
			RETURNING calls`, userID).Scan(&calls)
	})
	return calls, err
}

func (r *usageRepository) ReserveFree(ctx context.Context, userID string) (int, error) {
	var calls int
	err := retry(func() error {
		return r.pool.QueryRow(ctx, `
			INSERT INTO free_decisions (user_id, calls) VALUES ($1, 1)
			ON CONFLICT (user_id) DO UPDATE SET calls = free_decisions.calls + 1
			RETURNING calls`, userID).Scan(&calls)
	})
	return calls, err
}
