// Command seed loads the content bundled in the app (exported by the app's
// ContentSeedExport test into content-seed/<collection>/<lang>.json) into the
// admin page's tables, through the same validation the page uses.
//
//	go run ./cmd/seed -host <dsql endpoint> -dir ../holy_messages/content-seed -collection word_of_day
//
// It refuses a collection/language that already has items, so it can never
// overwrite what was edited in the admin page.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	dsqlauth "github.com/aws/aws-sdk-go-v2/feature/dsql/auth"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/reangeline/missale-backend/internal/adapters/outbound/persistence/dsql"
	"github.com/reangeline/missale-backend/internal/application/service"
	"github.com/reangeline/missale-backend/internal/core/domain"
)

func main() {
	host := flag.String("host", "", "DSQL endpoint")
	dir := flag.String("dir", "", "content-seed directory")
	collection := flag.String("collection", "", "collection to load")
	flag.Parse()
	if *host == "" || *dir == "" || *collection == "" {
		log.Fatal("-host, -dir and -collection are required")
	}
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		log.Fatal(err)
	}
	token, err := dsqlauth.GenerateDBConnectAdminAuthToken(ctx, *host, "us-east-1", cfg.Credentials)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, fmt.Sprintf("host=%s port=5432 user=admin dbname=postgres sslmode=verify-full password=%s", *host, token))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := dsql.NewContentRepository(pool)
	content := service.NewContentService(repo, nil)
	seeder := domain.Admin{Email: "seed (conteúdo embutido no app)"}

	for _, lang := range domain.ContentLanguages {
		path := filepath.Join(*dir, *collection, lang+".json")
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			fmt.Printf("%s: sem arquivo, pulado\n", lang)
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		existing, err := content.List(ctx, *collection, lang)
		if err != nil {
			log.Fatal(err)
		}
		if len(existing) > 0 {
			fmt.Printf("%s: já tem %d itens no painel, não sobrescrevo\n", lang, len(existing))
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		for i, item := range items {
			var head struct{ ID string }
			if err := json.Unmarshal(item, &head); err != nil {
				log.Fatalf("%s item %d: %v", lang, i, err)
			}
			if _, err := content.Save(ctx, seeder, *collection, lang, head.ID, item, i); err != nil {
				log.Fatalf("%s %s: %v", lang, head.ID, err)
			}
		}
		fmt.Printf("%s: %d itens carregados\n", lang, len(items))
	}
}
