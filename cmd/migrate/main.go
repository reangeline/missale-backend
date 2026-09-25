// Command migrate applies migrations/*.sql to an Aurora DSQL cluster as admin.
//
//	go run ./cmd/migrate -host <endpoint> -lambda-role <arn> [-from 002]
//
// DSQL runs one DDL per transaction, so each statement is sent on its own.
// Rerunning is safe: "already exists" (42710) is reported and skipped.
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
		for _, stmt := range statements(strings.ReplaceAll(string(raw), "{{LAMBDA_ROLE_ARN}}", *role)) {
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
