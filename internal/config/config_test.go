package config

import (
	"testing"

	"github.com/getsentry/sentry-go"
)

func TestInitSentryNoOpWithEmptyDSN(t *testing.T) {
	t.Setenv("TEST_SENTRY_DSN", "")

	InitSentry("TEST_SENTRY_DSN")

	if sentry.CurrentHub().Client() != nil {
		t.Fatal("InitSentry with empty DSN should not initialise a Sentry client")
	}
}

func TestResolveAppScheme(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{"empty falls back to the production scheme", "", DefaultAppScheme},
		{"production scheme is honoured", "foam", "foam"},
		{"internal variant scheme is honoured", "foam-internal", "foam-internal"},
		{"dev variant scheme is honoured", "foam-dev", "foam-dev"},
		{"unknown scheme falls back to the production scheme", "foam-evil", DefaultAppScheme},
		{"arbitrary url falls back to the production scheme", "https://evil.example", DefaultAppScheme},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveAppScheme(tt.requested); got != tt.want {
				t.Fatalf("ResolveAppScheme(%q) = %q, want %q", tt.requested, got, tt.want)
			}
		})
	}
}
