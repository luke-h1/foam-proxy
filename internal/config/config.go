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
	DefaultAppScheme     = "foam"
)

var allowedAppSchemes = map[string]bool{
	"foam":            true,
	"foam-dev":        true,
	"foam-internal":   true,
	"foam-testflight": true,
	"foam-e2e":        true,
}

func IsAllowedAppScheme(scheme string) bool {
	return allowedAppSchemes[scheme]
}

func ResolveAppScheme(requested string) string {
	if IsAllowedAppScheme(requested) {
		return requested
	}
	return DefaultAppScheme
}

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
	// MagicLink is the env-var fallback blob (MAGIC_LINK_BLOB), used for local/dev
	// and tests. In deployed environments the canonical blob lives in SSM and is
	// read at request time via MagicLinkSSMParam.
	MagicLink       *MagicLink
	MagicLinkAPIKey string
	// MagicLinkSSMParam is the SSM parameter name holding the canonical token blob.
	// Empty falls back to MagicLink (the env var).
	MagicLinkSSMParam string
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
		MagicLink:          ParseMagicLink(os.Getenv("MAGIC_LINK_BLOB")),
		MagicLinkAPIKey:    os.Getenv("MAGIC_LINK_API_KEY"),
		MagicLinkSSMParam:  os.Getenv("MAGIC_LINK_SSM_PARAM"),
	}, nil
}

// ParseMagicLink decodes a token blob JSON, returning nil for empty/invalid input
// or when the required tokens are missing (which disables the magic link).
func ParseMagicLink(raw string) *MagicLink {
	if raw == "" {
		return nil
	}

	var magic MagicLink
	if err := json.Unmarshal([]byte(raw), &magic); err != nil {
		log.Printf("failed to parse magic link blob: %v", err)
		return nil
	}

	if magic.AccessToken == "" || magic.RefreshToken == "" {
		log.Print("magic link blob missing tokens; magic link disabled")
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
