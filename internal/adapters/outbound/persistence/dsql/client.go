// Package dsql persists the little the server remembers — accounts and daily
// Jev usage — in Aurora DSQL. What users write is never stored.
package dsql

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

// Open connects to Aurora DSQL. DSQL only takes IAM auth: each new connection
// gets a short-lived token signed with the Lambda's credentials as password.
func Open(ctx context.Context, cfg aws.Config, endpoint, region string) (*pgxpool.Pool, error) {
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
	return pgxpool.NewWithConfig(ctx, pc)
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
