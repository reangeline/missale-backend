package dsql

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type userRepository struct{ pool *pgxpool.Pool }

func NewUserRepository(pool *pgxpool.Pool) outbound.UserRepository {
	return &userRepository{pool: pool}
}

func (r *userRepository) Save(ctx context.Context, u domain.User) error {
	return retry(func() error {
		_, err := r.pool.Exec(ctx,
			`INSERT INTO users (id, apple_sub) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`, u.ID, u.AppleSub)
		return err
	})
}

func (r *userRepository) Delete(ctx context.Context, userID string) error {
	return retry(func() error {
		return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM decision_usage WHERE user_id = $1`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM free_decisions WHERE user_id = $1`, userID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
			return err
		})
	})
}
