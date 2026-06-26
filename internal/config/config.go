package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	defaultTwitchTimeout = 20 * time.Second
	// DefaultAppScheme is the production app's custom URL scheme, used when a
	// deployment doesn't set APP_SCHEME.
	DefaultAppScheme = "foam"
)

// allowedAppSchemes are the custom URL schemes the Foam app registers, one per
// build variant (see app.config.ts). Redirects are restricted to these so the
// magic/proxy routes can't be used to bounce a token into an arbitrary scheme.
var allowedAppSchemes = map[string]bool{
	"foam":            true,
	"foam-dev":        true,
	"foam-internal":   true,
	"foam-testflight": true,
	"foam-e2e":        true,
}

// IsAllowedAppScheme reports whether scheme is a known Foam variant scheme.
func IsAllowedAppScheme(scheme string) bool {
	return allowedAppSchemes[scheme]
}

// ResolveAppScheme returns requested when it's a known variant scheme, otherwise
// the production default. requested comes from the untrusted ?scheme query param,
// so the allowlist stops the redirect from targeting an arbitrary scheme.
func ResolveAppScheme(requested string) string {
	if IsAllowedAppScheme(requested) {
		return requested
	}
	return DefaultAppScheme
}

// MagicLink is the stored test-account session the App Review magic link injects
// into the app, bypassing the OAuth/2FA flow Apple's reviewers cannot complete.
type MagicLink struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type Proxy struct {
	TwitchClientID     string
	TwitchClientSecret string
	TwitchTimeout      time.Duration
	DeployedBy         string
	DeployedAt         string
	GitSHA             string
	MagicLink          *MagicLink
	// MagicLinkAPIKey is the secret the magic route's ?key is checked against; empty disables it.
	MagicLinkAPIKey string
}

func LoadEnv() (*Proxy, error) {
	clientId := os.Getenv("TWITCH_CLIENT_ID")
	clientSecret := os.Getenv("TWITCH_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return nil, fmt.Errorf("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}

	return &Proxy{
		TwitchClientID:     clientId,
		TwitchClientSecret: clientSecret,
		TwitchTimeout:      defaultTwitchTimeout,
		DeployedBy:         os.Getenv("DEPLOYED_BY"),
		DeployedAt:         os.Getenv("DEPLOYED_AT"),
		GitSHA:             os.Getenv("GIT_SHA"),
		MagicLink:          parseMagicLink(os.Getenv("MAGIC_LINK_BLOB")),
		MagicLinkAPIKey:    os.Getenv("MAGIC_LINK_API_KEY"),
	}, nil
}

// parseMagicLink reads the optional MAGIC_LINK_BLOB env var; a missing or invalid
// blob disables the magic link rather than failing boot.
func parseMagicLink(raw string) *MagicLink {
	if raw == "" {
		return nil
	}

	var magic MagicLink
	if err := json.Unmarshal([]byte(raw), &magic); err != nil {
		log.Printf("failed to parse MAGIC_LINK_BLOB: %v", err)
		return nil
	}

	if magic.AccessToken == "" || magic.RefreshToken == "" {
		log.Print("MAGIC_LINK_BLOB missing tokens; magic link disabled")
		return nil
	}

	return &magic
}

func SentryOptions(dsn string) sentry.ClientOptions {
	return sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
		Release:          os.Getenv("SENTRY_RELEASE"),
		AttachStacktrace: true,
		SampleRate:       1.0,
		EnableTracing:    true,
		TracesSampleRate: 0.5,
		MaxBreadcrumbs:   50,
		SendDefaultPII:   false,
		Debug:            false,
	}
}
