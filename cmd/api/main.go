package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/reangeline/missale-backend/internal/auth"
	"github.com/reangeline/missale-backend/internal/config"
	"github.com/reangeline/missale-backend/internal/httpapi"
	"github.com/reangeline/missale-backend/internal/jev"
	"github.com/reangeline/missale-backend/internal/store"
	"github.com/reangeline/missale-backend/internal/subscription"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		log.Fatal(err)
	}
	db, err := store.Open(ctx, awsCfg, cfg.DSQLEndpoint, cfg.AWSRegion)
	if err != nil {
		log.Fatal(err)
	}
	subs, err := subscription.NewVerifier(cfg.BundleID, cfg.ProductIDs)
	if err != nil {
		log.Fatal(err)
	}
	api := &httpapi.API{
		Apple:         auth.NewAppleVerifier(cfg.BundleID),
		Sessions:      auth.NewCognito(awsCfg, cfg.AWSRegion, cfg.CognitoUserPoolID, cfg.CognitoClientID),
		Store:         db,
		Subscriptions: subs,
		Jev:           jev.NewClient(cfg.OpenRouterAPIKey, cfg.JevModel),
		DailyLimit:    cfg.DailyDecisionLimit,
		Log:           slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		log.Println("listening on :8080")
		log.Fatal(http.ListenAndServe(":8080", api.Router()))
	}
	lambda.Start(httpadapter.NewV2(api.Router()).ProxyWithContext)
}
