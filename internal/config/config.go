package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	defaultTwitchTimeout = 20
)

var allowedApps = map[string]appEnvVarNames{
	"foam-app": {
		clientID:     "TWITCH_CLIENT_ID_APP",
		clientSecret: "TWITCH_CLIENT_SECRET_APP",
		redirectURI:  "REDIRECT_URI_FOAM_APP",
	},
	"foam-menubar": {
		clientID:     "TWITCH_CLIENT_ID_MENUBAR",
		clientSecret: "TWITCH_CLIENT_SECRET_MENUBAR",
		redirectURI:  "REDIRECT_URI_MENUBAR",
	},
}

type Proxy struct {
	Apps          map[string]AppConfig
	TwitchTimeout time.Duration
	DeployedBy    string
	DeployedAt    string
	GitSHA        string
}

type AppConfig struct {
	TwitchClientID     string
	TwitchClientSecret string
	RedirectURI        string
}

func LoadEnv() (*Proxy, error) {
	appNames := parseAppNames(os.Getenv("PROXY_APPS"))

	if len(appNames) == 0 {
		return nil, fmt.Errorf("PROXY_APPS is required")
	}

	apps := make(map[string]AppConfig, len(appNames))
	seenAppKeys := make(map[string]string, len(appNames))

	for _, appName := range appNames {
		envNames, err := appEnvNames(appName)
		if err != nil {
			return nil, err
		}

		appKey := appConfigKey(envNames)
		if prior, ok := seenAppKeys[appKey]; ok {
			return nil, fmt.Errorf("duplicate app configuration after normalization: %q conflicts with %q", prior, appName)
		}
		seenAppKeys[appKey] = appName

		clientID := os.Getenv(envNames.clientID)
		clientSecret := os.Getenv(envNames.clientSecret)
		redirectURI := os.Getenv(envNames.redirectURI)

		if clientID == "" || clientSecret == "" || redirectURI == "" {
			return nil, fmt.Errorf("%s, %s and %s are required for app %q", envNames.clientID, envNames.clientSecret, envNames.redirectURI, appName)
		}

		apps[appName] = AppConfig{
			TwitchClientID:     clientID,
			TwitchClientSecret: clientSecret,
			RedirectURI:        redirectURI,
		}
	}

	return &Proxy{
		Apps:          apps,
		TwitchTimeout: defaultTwitchTimeout,
		DeployedBy:    os.Getenv("DEPLOYED_BY"),
		DeployedAt:    os.Getenv("DEPLOYED_AT"),
		GitSHA:        os.Getenv("GIT_SHA"),
	}, nil
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

func parseAppNames(raw string) []string {
	parts := strings.Split(raw, ",")
	apps := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		apps = append(apps, name)
	}
	return apps
}

func appConfigKey(envNames appEnvVarNames) string {
	return strings.Join([]string{envNames.clientID, envNames.clientSecret, envNames.redirectURI}, "|")
}

type appEnvVarNames struct {
	clientID     string
	clientSecret string
	redirectURI  string
}

func appEnvNames(appName string) (appEnvVarNames, error) {
	appName = strings.TrimSpace(appName)
	envNames, ok := allowedApps[appName]
	if !ok {
		return appEnvVarNames{}, fmt.Errorf("unsupported app %q: only foam-app and foam-menubar are allowed", appName)
	}
	return envNames, nil
}
