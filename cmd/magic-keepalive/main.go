package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/magickeepalive"
	"github.com/foam/proxy/internal/magiclink"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

const twitchTimeout = 20 * time.Second

func main() {
	initSentry()
	lambda.Start(handle)
}

// handle runs on an EventBridge schedule. It is a no-op unless
// REVIEWER_ACCOUNT_REFRESH_ENABLED is true, so the token is only kept warm (and
// SSM only touched) during App Review windows.
func handle(ctx context.Context) error {
	if !enabled() {
		log.Print("REVIEWER_ACCOUNT_REFRESH_ENABLED is off; skipping refresh")
		return nil
	}

	if err := run(ctx); err != nil {
		log.Printf("magic link keepalive failed: %v", err)
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)
		return err
	}

	log.Print("refreshed magic link keepalive token")
	return nil
}

func run(ctx context.Context) error {
	param := os.Getenv("MAGIC_LINK_SSM_PARAM")
	if param == "" {
		return fmt.Errorf("MAGIC_LINK_SSM_PARAM is required")
	}

	clientID := os.Getenv("TWITCH_CLIENT_ID")
	clientSecret := os.Getenv("TWITCH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}

	store, err := magiclink.NewStore(ctx, param)
	if err != nil {
		return fmt.Errorf("init ssm store: %w", err)
	}

	twitch := services.NewTwitchService(clientID, clientSecret, twitchTimeout)
	return magickeepalive.New(store, twitch).Refresh(ctx)
}

func enabled() bool {
	v, _ := strconv.ParseBool(os.Getenv("REVIEWER_ACCOUNT_REFRESH_ENABLED"))
	return v
}

func initSentry() {
	dsn := os.Getenv("REFRESH_DSN")
	if dsn == "" {
		return
	}
	if err := sentry.Init(config.SentryOptions(dsn)); err != nil {
		log.Printf("sentry init: %v", err)
	}
}
