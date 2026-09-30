package dsql

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type usageRepository struct{ pool *pgxpool.Pool }

func NewUsageRepository(pool *pgxpool.Pool) outbound.UsageRepository {
	return &usageRepository{pool: pool}
}

// Each statement only counts while the limit ($2) is not reached yet: at the
// limit the conflicting row is left alone and nothing is returned, so a
// refused call never raises the counter. The INSERT branch (first call ever,
// or first today) always writes 1, hence the limit > 0 check in Reserve.
const (
	reserveFree = `
		INSERT INTO free_decisions (user_id, calls) VALUES ($1, 1)
		ON CONFLICT (user_id) DO UPDATE SET calls = free_decisions.calls + 1
		WHERE free_decisions.calls < $2
		RETURNING calls`
	reserveDaily = `
		INSERT INTO decision_usage (user_id, day, calls) VALUES ($1, CURRENT_DATE, 1)
		ON CONFLICT (user_id, day) DO UPDATE SET calls = decision_usage.calls + 1
		WHERE decision_usage.calls < $2
		RETURNING calls`
)

func (r *usageRepository) Reserve(ctx context.Context, res outbound.Reservation) error {
	if res.SpendFree && res.FreeLimit <= 0 {
		return domain.ErrNotSubscribed
	}
	if res.DailyLimit <= 0 {
		return domain.ErrDailyLimit
	}
	// One transaction, so a refusal by the second limit rolls back the first.
	// A lost race (40001 at commit) reruns the whole transaction and the
	// limits are checked again against the winner's counts.
	return retry(func() error {
		return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
			if res.SpendFree {
				if err := spend(ctx, tx, reserveFree, res.UserID, res.FreeLimit, domain.ErrNotSubscribed); err != nil {
					return err
				}
			}
			return spend(ctx, tx, reserveDaily, res.UserID, res.DailyLimit, domain.ErrDailyLimit)
		})
	})
}

func spend(ctx context.Context, tx pgx.Tx, sql, userID string, limit int, refused error) error {
	var calls int
	err := tx.QueryRow(ctx, sql, userID, limit).Scan(&calls)
	if errors.Is(err, pgx.ErrNoRows) {
		return refused
	}
	return err
}
