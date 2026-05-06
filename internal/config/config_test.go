package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadEnvBuildsAppConfigMap(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app,foam-menubar")
	t.Setenv("TWITCH_CLIENT_ID_APP", "app-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "app-secret")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")
	t.Setenv("TWITCH_CLIENT_ID_MENUBAR", "menubar-id")
	t.Setenv("TWITCH_CLIENT_SECRET_MENUBAR", "menubar-secret")
	t.Setenv("REDIRECT_URI_MENUBAR", "foam-menubar://")
	t.Setenv("DEPLOYED_BY", "ci")
	t.Setenv("DEPLOYED_AT", "2026-04-20T10:00:00Z")
	t.Setenv("GIT_SHA", "abc123")

	cfg, warnings, err := LoadEnv()
	if err != nil {
		t.Fatalf("LoadEnv() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v, want none", warnings)
	}

	all := cfg.Apps.All()
	if got, want := len(all), 2; got != want {
		t.Fatalf("len(cfg.Apps.All()) = %d, want %d", got, want)
	}

	if got, want := cfg.Apps.Names(), []string{"foam-app", "foam-menubar"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("cfg.Apps.Names() = %v, want %v", got, want)
	}

	foamApp, err := cfg.Apps.Get("foam-app")
	if err != nil {
		t.Fatalf("cfg.Apps.Get(foam-app) error = %v", err)
	}
	if got := foamApp.Config; got.TwitchClientID != "app-id" || got.TwitchClientSecret != "app-secret" || got.RedirectURI != "foam://" {
		t.Fatalf("foam-app config = %+v", got)
	}

	menubarApp, err := cfg.Apps.Get("foam-menubar")
	if err != nil {
		t.Fatalf("cfg.Apps.Get(foam-menubar) error = %v", err)
	}
	if got := menubarApp.Config; got.TwitchClientID != "menubar-id" || got.TwitchClientSecret != "menubar-secret" || got.RedirectURI != "foam-menubar://" {
		t.Fatalf("foam-menubar config = %+v", got)
	}

	if cfg.DeployedBy != "ci" || cfg.DeployedAt != "2026-04-20T10:00:00Z" || cfg.GitSHA != "abc123" {
		t.Fatalf("deployment metadata not preserved: %+v", cfg)
	}
}

func TestLoadEnvErrorsWhenAppConfigMissing(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app")
	t.Setenv("TWITCH_CLIENT_ID_APP", "app-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")

	_, _, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "foam-app") {
		t.Fatalf("LoadEnv() error = %q, want app name", err)
	}
}

func TestLoadEnvErrorsWhenAppsListMissing(t *testing.T) {
	_ = os.Unsetenv("PROXY_APPS")

	_, _, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "PROXY_APPS") {
		t.Fatalf("LoadEnv() error = %q, want PROXY_APPS message", err)
	}
}

func TestLoadEnvDeduplicatesAppsAndReturnsWarning(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app,foam-app")
	t.Setenv("TWITCH_CLIENT_ID_APP", "shared-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "shared-secret")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")

	cfg, warnings, err := LoadEnv()
	if err != nil {
		t.Fatalf("LoadEnv() error = %v", err)
	}

	if got, want := cfg.Apps.Names(), []string{"foam-app"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("cfg.Apps.Names() = %v, want %v", got, want)
	}

	if got, want := len(warnings), 1; got != want {
		t.Fatalf("len(warnings) = %d, want %d", got, want)
	}

	if warnings[0].Code != WarningDuplicateApp || warnings[0].App != "foam-app" {
		t.Fatalf("warnings[0] = %+v", warnings[0])
	}
}

func TestLoadEnvRejectsUnsupportedApps(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app,other-app")
	t.Setenv("TWITCH_CLIENT_ID_APP", "app-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "app-secret")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")

	_, _, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "unsupported app") {
		t.Fatalf("LoadEnv() error = %q, want unsupported app message", err)
	}
}

func TestAppCapabilitiesGetReturnsTypedErrors(t *testing.T) {
	apps := NewAppCapabilities([]ConfiguredApp{
		{
			Name: "foam-app",
			Config: AppConfig{
				TwitchClientID:     "app-id",
				TwitchClientSecret: "app-secret",
				RedirectURI:        "foam://",
			},
		},
	})

	_, err := apps.Get("")
	if err == nil {
		t.Fatal("apps.Get(\"\") error = nil, want error")
	}
	missingErr, ok := err.(*AppLookupError)
	if !ok || missingErr.Code != AppLookupMissingApp {
		t.Fatalf("apps.Get(\"\") error = %#v, want AppLookupMissingApp", err)
	}

	_, err = apps.Get("other-app")
	if err == nil {
		t.Fatal("apps.Get(other-app) error = nil, want error")
	}
	invalidErr, ok := err.(*AppLookupError)
	if !ok || invalidErr.Code != AppLookupInvalidApp || invalidErr.App != "other-app" {
		t.Fatalf("apps.Get(other-app) error = %#v, want AppLookupInvalidApp", err)
	}
}
