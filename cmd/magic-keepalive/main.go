package main

import (
	"context"
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/magickeepalive"
	"github.com/getsentry/sentry-go"
)

func main() {
	config.InitSentry("REFRESH_DSN")
	lambda.Start(handle)
}

const monitorSlug = "magic-link-keepalive"

func keepaliveMonitor() *sentry.MonitorConfig {
	return &sentry.MonitorConfig{
		Schedule:              sentry.IntervalSchedule(3, sentry.MonitorScheduleUnitHour),
		MaxRuntime:            5,
		CheckInMargin:         10,
		FailureIssueThreshold: 1,
	}
}

func finishCheckIn(id *sentry.EventID, monitor *sentry.MonitorConfig, status sentry.CheckInStatus) {
	if id == nil {
		return
	}
	sentry.CaptureCheckIn(&sentry.CheckIn{
		ID:          *id,
		MonitorSlug: monitorSlug,
		Status:      status,
	}, monitor)
}

// handle runs on an EventBridge schedule. It is a no-op unless
// REVIEWER_ACCOUNT_REFRESH_ENABLED is true, so the token is only kept warm (and
// SSM only touched) during App Review windows.
func handle(ctx context.Context) error {
	if !enabled() {
		log.Print("REVIEWER_ACCOUNT_REFRESH_ENABLED is off; skipping refresh")
		return nil
	}

	monitor := keepaliveMonitor()
	checkInID := sentry.CaptureCheckIn(&sentry.CheckIn{
		MonitorSlug: monitorSlug,
		Status:      sentry.CheckInStatusInProgress,
	}, monitor)
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetContext("monitor", sentry.Context{"slug": monitorSlug})
	})

	if err := run(ctx); err != nil {
		log.Printf("magic link keepalive failed: %v", err)
		// Already rotated: expected, non-retryable. Close the check-in OK and return
		// nil so EventBridge doesn't re-invoke. Pre-refresh failures stay retryable.
		if errors.Is(err, magickeepalive.ErrTokenRotated) {
			finishCheckIn(checkInID, monitor, sentry.CheckInStatusOK)
			sentry.Flush(2 * time.Second)
			return nil
		}
		finishCheckIn(checkInID, monitor, sentry.CheckInStatusError)
		sentry.CaptureException(err)
		sentry.Flush(2 * time.Second)
		return err
	}

	finishCheckIn(checkInID, monitor, sentry.CheckInStatusOK)
	sentry.Flush(2 * time.Second)
	log.Print("refreshed magic link keepalive token")
	return nil
}

func run(ctx context.Context) error {
	refresher, err := magickeepalive.NewFromEnv(ctx, os.Getenv)
	if err != nil {
		return err
	}
	return refresher.Refresh(ctx)
}

func enabled() bool {
	raw := os.Getenv("REVIEWER_ACCOUNT_REFRESH_ENABLED")
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("invalid REVIEWER_ACCOUNT_REFRESH_ENABLED %q; treating as disabled: %v", raw, err)
		return false
	}
	return v
}
