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

	cfg, err := LoadEnv()
	if err != nil {
		t.Fatalf("LoadEnv() error = %v", err)
	}

	if got, want := len(cfg.Apps), 2; got != want {
		t.Fatalf("len(cfg.Apps) = %d, want %d", got, want)
	}

	if got := cfg.Apps["foam-app"]; got.TwitchClientID != "app-id" || got.TwitchClientSecret != "app-secret" || got.RedirectURI != "foam://" {
		t.Fatalf("foam-app config = %+v", got)
	}

	if got := cfg.Apps["foam-menubar"]; got.TwitchClientID != "menubar-id" || got.TwitchClientSecret != "menubar-secret" || got.RedirectURI != "foam-menubar://" {
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

	_, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "foam-app") {
		t.Fatalf("LoadEnv() error = %q, want app name", err)
	}
}

func TestLoadEnvErrorsWhenAppsListMissing(t *testing.T) {
	_ = os.Unsetenv("PROXY_APPS")

	_, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "PROXY_APPS") {
		t.Fatalf("LoadEnv() error = %q, want PROXY_APPS message", err)
	}
}

func TestLoadEnvErrorsWhenAppsCollideAfterNormalization(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app,foam-app")
	t.Setenv("TWITCH_CLIENT_ID_APP", "shared-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "shared-secret")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")

	_, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("LoadEnv() error = %q, want duplicate message", err)
	}
}

func TestLoadEnvRejectsUnsupportedApps(t *testing.T) {
	t.Setenv("PROXY_APPS", "foam-app,other-app")
	t.Setenv("TWITCH_CLIENT_ID_APP", "app-id")
	t.Setenv("TWITCH_CLIENT_SECRET_APP", "app-secret")
	t.Setenv("REDIRECT_URI_FOAM_APP", "foam://")

	_, err := LoadEnv()
	if err == nil {
		t.Fatal("LoadEnv() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "unsupported app") {
		t.Fatalf("LoadEnv() error = %q, want unsupported app message", err)
	}
}
