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

	httpAdapter "github.com/reangeline/missale-backend/internal/adapters/inbound/http"
	"github.com/reangeline/missale-backend/internal/adapters/outbound/auth/apple"
	"github.com/reangeline/missale-backend/internal/adapters/outbound/auth/cognito"
	"github.com/reangeline/missale-backend/internal/adapters/outbound/decision/jev"
	"github.com/reangeline/missale-backend/internal/adapters/outbound/persistence/dsql"
	s3storage "github.com/reangeline/missale-backend/internal/adapters/outbound/storage/s3"
	"github.com/reangeline/missale-backend/internal/adapters/outbound/subscription/storekit"
	appservice "github.com/reangeline/missale-backend/internal/application/service"
	appconfig "github.com/reangeline/missale-backend/pkg/config"
)

func main() {
	ctx := context.Background()
	cfg, err := appconfig.Load()
	if err != nil {
		log.Fatal(err)
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		log.Fatal(err)
	}

	// Outbound adapters
	pool, err := dsql.Open(ctx, awsCfg, cfg.DSQLEndpoint, cfg.AWSRegion)
	if err != nil {
		log.Fatal(err)
	}
	userRepo := dsql.NewUserRepository(pool)
	usageRepo := dsql.NewUsageRepository(pool)
	identityVerifier := apple.NewIdentityVerifier(cfg.BundleID)
	authProvider := cognito.NewAuthProvider(awsCfg, cfg.AWSRegion, cfg.CognitoUserPoolID, cfg.CognitoClientID)
	subscriptionVerifier, err := storekit.NewVerifier(cfg.BundleID, cfg.ProductIDs)
	if err != nil {
		log.Fatal(err)
	}
	decisionEngine := jev.NewClient(cfg.OpenRouterAPIKey, cfg.JevModel)

	// Application services
	authService := appservice.NewAuthService(identityVerifier, authProvider, userRepo)
	accountService := appservice.NewAccountService(authProvider, userRepo)
	decisionService := appservice.NewDecisionService(subscriptionVerifier, usageRepo, decisionEngine, cfg.DailyDecisionLimit, cfg.FreeDecisions)

	var adminRoutes *httpAdapter.AdminRoutes
	if cfg.CognitoAdminClientID != "" && cfg.ContentBucket != "" {
		adminRoutes = &httpAdapter.AdminRoutes{
			Auth: appservice.NewAdminAuthService(
				cognito.NewAdminAuthProvider(awsCfg, cfg.AWSRegion, cfg.CognitoUserPoolID, cfg.CognitoAdminClientID)),
			Content: appservice.NewContentService(
				dsql.NewContentRepository(pool), s3storage.NewPublisher(awsCfg, cfg.ContentBucket)),
			Origins: cfg.AdminOrigins,
		}
	}

	// Inbound adapter
	router := httpAdapter.NewRouter(authService, accountService, decisionService, adminRoutes,
		slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		log.Println("listening on :8080")
		log.Fatal(http.ListenAndServe(":8080", router))
	}
	lambda.Start(httpadapter.NewV2(router).ProxyWithContext)
}
