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

func TestSentryReleasePrefersEnvThenGitSHA(t *testing.T) {
	t.Run("SENTRY_RELEASE wins", func(t *testing.T) {
		t.Setenv("SENTRY_RELEASE", "v1.2.3")
		t.Setenv("GIT_SHA", "abc123")
		if got := sentryRelease(); got != "v1.2.3" {
			t.Fatalf("sentryRelease() = %q, want v1.2.3", got)
		}
	})
	t.Run("falls back to GIT_SHA", func(t *testing.T) {
		t.Setenv("SENTRY_RELEASE", "")
		t.Setenv("GIT_SHA", "abc123")
		if got := sentryRelease(); got != "abc123" {
			t.Fatalf("sentryRelease() = %q, want abc123", got)
		}
	})
}

func TestSentryOptionsCarriesReleaseTagsAndMetrics(t *testing.T) {
	t.Setenv("SENTRY_RELEASE", "")
	t.Setenv("GIT_SHA", "deadbeef")
	t.Setenv("DEPLOYED_BY", "ci")
	t.Setenv("DEPLOYED_AT", "")

	opts := SentryOptions("https://example@sentry.io/1")

	if opts.Release != "deadbeef" {
		t.Fatalf("Release = %q, want deadbeef (source context needs a commit-linked release)", opts.Release)
	}
	if opts.DisableMetrics {
		t.Fatal("DisableMetrics should be false so the Meter API emits APM metrics")
	}
	if opts.Tags["git_sha"] != "deadbeef" || opts.Tags["deployed_by"] != "ci" {
		t.Fatalf("default tags = %v, want git_sha/deployed_by populated", opts.Tags)
	}
	if _, ok := opts.Tags["deployed_at"]; ok {
		t.Fatalf("empty env vars should be omitted from tags, got %v", opts.Tags)
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
