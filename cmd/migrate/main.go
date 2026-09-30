// Command migrate applies migrations/*.sql to an Aurora DSQL cluster as admin.
//
//	go run ./cmd/migrate -host <endpoint> -lambda-role <arn> [-from 002]
//
// DSQL runs one DDL per transaction, so each statement is sent on its own.
// Every file runs again on each deploy, so it must be idempotent: "already
// exists" (42710) is reported and skipped.
//
// A file named *.once.sql is the exception, for data changes that must not
// repeat (a reset, say): it runs once per cluster, all its statements and
// its entry in schema_once in one transaction, and is skipped from then on.
// DSQL keeps DDL out of such a transaction, so a once file holds DML only.
// Its name is the key: renaming it runs it again.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dsqlauth "github.com/aws/aws-sdk-go-v2/feature/dsql/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func main() {
	host := flag.String("host", "", "DSQL endpoint")
	role := flag.String("lambda-role", "", "IAM role ARN of the API Lambda")
	region := flag.String("region", "us-east-1", "AWS region")
	from := flag.String("from", "", "apply only files named >= this prefix, e.g. 002")
	flag.Parse()
	if *host == "" || *role == "" {
		log.Fatal("-host and -lambda-role are required")
	}
	ctx := context.Background()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(*region))
	if err != nil {
		log.Fatal(err)
	}
	token, err := dsqlauth.GenerateDBConnectAdminAuthToken(ctx, *host, *region, awsCfg.Credentials)
	if err != nil {
		log.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, fmt.Sprintf("host=%s port=5432 user=admin dbname=postgres sslmode=verify-full password=%s", *host, token))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(ctx)

	files, _ := filepath.Glob("migrations/*.sql")
	sort.Strings(files)
	for _, f := range files {
		if *from != "" && filepath.Base(f) < *from {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("==", f)
		stmts := statements(strings.ReplaceAll(string(raw), "{{LAMBDA_ROLE_ARN}}", *role))
		if isOnce(f) {
			if err := runOnce(ctx, conn, filepath.Base(f), stmts); err != nil {
				log.Fatalf("  %s: %v", f, err)
			}
			continue
		}
		for _, stmt := range stmts {
			_, err := conn.Exec(ctx, stmt)
			var pgErr *pgconn.PgError
			switch {
			case err == nil:
				fmt.Println("  ok:", firstLine(stmt))
			case errors.As(err, &pgErr) && pgErr.Code == "42710":
				fmt.Println("  exists:", firstLine(stmt))
			default:
				log.Fatalf("  %s\n  %v", firstLine(stmt), err)
			}
		}
	}
}

func isOnce(file string) bool { return strings.HasSuffix(file, ".once.sql") }

// runOnce applies a once file unless schema_once already lists it. The
// statements and the entry commit together: a failure leaves neither, and a
// second concurrent run fails on the primary key instead of repeating it.
func runOnce(ctx context.Context, conn *pgx.Conn, name string, stmts []string) error {
	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_once (
		name       text        PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("schema_once: %w", err)
	}
	var done bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_once WHERE name = $1)`, name).Scan(&done); err != nil {
		return err
	}
	if done {
		fmt.Println("  already applied")
		return nil
	}
	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		for _, stmt := range stmts {
			tag, err := tx.Exec(ctx, stmt)
			if err != nil {
				return fmt.Errorf("%s: %w", firstLine(stmt), err)
			}
			fmt.Printf("  ok: %s (%d rows)\n", firstLine(stmt), tag.RowsAffected())
		}
		_, err := tx.Exec(ctx, `INSERT INTO schema_once (name) VALUES ($1)`, name)
		return err
	})
}

// statements splits on ";" at line end and drops comment-only chunks.
func statements(sql string) []string {
	var out []string
	for _, chunk := range strings.Split(sql, ";\n") {
		var lines []string
		for _, l := range strings.Split(chunk, "\n") {
			if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "--") {
				lines = append(lines, l)
			}
		}
		if s := strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";"); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstLine(s string) string { return strings.SplitN(s, "\n", 2)[0] }
