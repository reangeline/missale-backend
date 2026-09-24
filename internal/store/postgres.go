// Package store keeps the little the server remembers: which accounts exist
// and how many Jev calls each made per day. What users write is never stored.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	dsqlauth "github.com/aws/aws-sdk-go-v2/feature/dsql/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBRole is the Postgres role the Lambda logs in as; migrations/001 maps it
// to the Lambda's IAM role.
const DBRole = "missale_api"

type Store struct{ pool *pgxpool.Pool }

// Open connects to Aurora DSQL. DSQL only takes IAM auth: each new connection
// gets a short-lived token signed with the Lambda's credentials as password.
func Open(ctx context.Context, cfg aws.Config, endpoint, region string) (*Store, error) {
	pc, err := pgxpool.ParseConfig(fmt.Sprintf(
		"host=%s port=5432 user=%s dbname=postgres sslmode=verify-full", endpoint, DBRole))
	if err != nil {
		return nil, err
	}
	pc.MaxConns = 2
	pc.MaxConnLifetime = 50 * time.Minute // DSQL closes connections at 60 min
	pc.BeforeConnect = func(ctx context.Context, cc *pgx.ConnConfig) error {
		token, err := dsqlauth.GenerateDbConnectAuthToken(ctx, endpoint, region, cfg.Credentials)
		if err != nil {
			return fmt.Errorf("DSQL token: %w", err)
		}
		cc.Password = token
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// TouchUser records the account on sign-in; repeated sign-ins are no-ops.
func (s *Store) TouchUser(ctx context.Context, userID, appleSub string) error {
	return retry(func() error {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO users (id, apple_sub) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING`,
			userID, appleSub)
		return err
	})
}

// ReserveDecision counts one Jev call for today (UTC) and reports whether the
// user is still within the daily limit. Counting before the call means a
// failed Jev request still spends one — simpler, and the limit is generous.
func (s *Store) ReserveDecision(ctx context.Context, userID string, limit int) (bool, error) {
	var calls int
	err := retry(func() error {
		return s.pool.QueryRow(ctx, `
			INSERT INTO decision_usage (user_id, day, calls) VALUES ($1, CURRENT_DATE, 1)
			ON CONFLICT (user_id, day) DO UPDATE SET calls = decision_usage.calls + 1
			RETURNING calls`, userID).Scan(&calls)
	})
	if err != nil {
		return false, err
	}
	return calls <= limit, nil
}

// DeleteUser erases everything the server holds about the account.
func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	return retry(func() error {
		return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `DELETE FROM decision_usage WHERE user_id = $1`, userID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
			return err
		})
	})
}

// retry reruns a statement that lost an optimistic-concurrency race: DSQL
// reports those as serialization failures (40001) instead of blocking.
func retry(f func() error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = f(); err == nil {
			return nil
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "40001" {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
	}
	return err
}
