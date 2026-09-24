// Package config reads the Lambda environment once at cold start.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Environment       string
	AWSRegion         string
	CognitoUserPoolID string
	CognitoClientID   string
	DSQLEndpoint      string
	OpenRouterAPIKey  string
	JevModel          string
	// BundleID is both the audience of Apple identity tokens and the bundle
	// StoreKit transactions must belong to.
	BundleID string
	// ProductIDs are the subscriptions that unlock the orientação.
	ProductIDs []string
	// DailyDecisionLimit caps Jev calls per user per UTC day. One orientação
	// uses two calls (state + risk, then the reviewed reply).
	DailyDecisionLimit int
}

func Load() (Config, error) {
	c := Config{
		Environment:       getenv("ENVIRONMENT", "dev"),
		AWSRegion:         getenv("AWS_REGION", "us-east-1"),
		CognitoUserPoolID: os.Getenv("COGNITO_USER_POOL_ID"),
		CognitoClientID:   os.Getenv("COGNITO_CLIENT_ID"),
		DSQLEndpoint:      os.Getenv("DSQL_ENDPOINT"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		JevModel:          getenv("JEV_MODEL", "typesafe/jev-1.13"),
		BundleID:          getenv("APP_BUNDLE_ID", "com.holymessages.app"),
		ProductIDs:        []string{"mensal", "anual"},
	}
	limit, err := strconv.Atoi(getenv("DAILY_DECISION_LIMIT", "40"))
	if err != nil {
		return c, fmt.Errorf("DAILY_DECISION_LIMIT: %w", err)
	}
	c.DailyDecisionLimit = limit
	for name, value := range map[string]string{
		"COGNITO_USER_POOL_ID": c.CognitoUserPoolID,
		"COGNITO_CLIENT_ID":    c.CognitoClientID,
		"DSQL_ENDPOINT":        c.DSQLEndpoint,
		"OPENROUTER_API_KEY":   c.OpenRouterAPIKey,
	} {
		if value == "" {
			return c, fmt.Errorf("missing %s", name)
		}
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
