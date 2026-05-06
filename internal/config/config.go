package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

const (
	defaultTwitchTimeout = 20 * time.Second
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

type WarningCode string

const (
	WarningDuplicateApp WarningCode = "duplicate_app"
)

type Warning struct {
	Code    WarningCode
	App     string
	Message string
}

type AppLookupErrorCode string

const (
	AppLookupMissingApp AppLookupErrorCode = "missing_app"
	AppLookupInvalidApp AppLookupErrorCode = "invalid_app"
)

type AppLookupError struct {
	Code AppLookupErrorCode
	App  string
}

func (e *AppLookupError) Error() string {
	switch e.Code {
	case AppLookupMissingApp:
		return "missing app"
	case AppLookupInvalidApp:
		if e.App == "" {
			return "invalid app"
		}
		return fmt.Sprintf("invalid app %q", e.App)
	default:
		return "app lookup failed"
	}
}

type Proxy struct {
	Apps          AppCapabilities
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

type ConfiguredApp struct {
	Name   string
	Config AppConfig
}

type AppCapabilities struct {
	ordered []ConfiguredApp
	byName  map[string]ConfiguredApp
}

func NewAppCapabilities(apps []ConfiguredApp) AppCapabilities {
	ordered := make([]ConfiguredApp, len(apps))
	copy(ordered, apps)

	byName := make(map[string]ConfiguredApp, len(apps))
	for _, app := range ordered {
		byName[app.Name] = app
	}

	return AppCapabilities{
		ordered: ordered,
		byName:  byName,
	}
}

func (a AppCapabilities) All() []ConfiguredApp {
	out := make([]ConfiguredApp, len(a.ordered))
	copy(out, a.ordered)
	return out
}

func (a AppCapabilities) Names() []string {
	out := make([]string, 0, len(a.ordered))
	for _, app := range a.ordered {
		out = append(out, app.Name)
	}
	return out
}

func (a AppCapabilities) Get(app string) (ConfiguredApp, error) {
	app = strings.TrimSpace(app)
	if app == "" {
		return ConfiguredApp{}, &AppLookupError{Code: AppLookupMissingApp}
	}

	configuredApp, ok := a.byName[app]
	if !ok {
		return ConfiguredApp{}, &AppLookupError{Code: AppLookupInvalidApp, App: app}
	}

	return configuredApp, nil
}

func LoadEnv() (*Proxy, []Warning, error) {
	appNames := parseAppNames(os.Getenv("PROXY_APPS"))

	if len(appNames) == 0 {
		return nil, nil, fmt.Errorf("PROXY_APPS is required")
	}

	configuredApps := make([]ConfiguredApp, 0, len(appNames))
	seenApps := make(map[string]struct{}, len(appNames))
	warnings := make([]Warning, 0)

	for _, appName := range appNames {
		appName = strings.TrimSpace(appName)
		if _, ok := seenApps[appName]; ok {
			warnings = append(warnings, Warning{
				Code:    WarningDuplicateApp,
				App:     appName,
				Message: fmt.Sprintf("duplicate app %q ignored", appName),
			})
			continue
		}
		seenApps[appName] = struct{}{}

		envNames, err := appEnvNames(appName)
		if err != nil {
			return nil, warnings, err
		}

		clientID := os.Getenv(envNames.clientID)
		clientSecret := os.Getenv(envNames.clientSecret)
		redirectURI := os.Getenv(envNames.redirectURI)

		if clientID == "" || clientSecret == "" || redirectURI == "" {
			return nil, warnings, fmt.Errorf("%s, %s and %s are required for app %q", envNames.clientID, envNames.clientSecret, envNames.redirectURI, appName)
		}

		configuredApps = append(configuredApps, ConfiguredApp{
			Name: appName,
			Config: AppConfig{
				TwitchClientID:     clientID,
				TwitchClientSecret: clientSecret,
				RedirectURI:        redirectURI,
			},
		})
	}

	return &Proxy{
		Apps:          NewAppCapabilities(configuredApps),
		TwitchTimeout: defaultTwitchTimeout,
		DeployedBy:    os.Getenv("DEPLOYED_BY"),
		DeployedAt:    os.Getenv("DEPLOYED_AT"),
		GitSHA:        os.Getenv("GIT_SHA"),
	}, warnings, nil
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
