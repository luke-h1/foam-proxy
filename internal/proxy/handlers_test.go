package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

type fakeTwitchService struct {
	defaultTokenCalls int
	refreshTokenCalls int
	lastRefreshToken  string
	defaultTokenErr   error
	refreshTokenErr   error
}

func (f *fakeTwitchService) DefaultToken(context.Context) (*services.TwitchTokenResponse, error) {
	f.defaultTokenCalls++
	if f.defaultTokenErr != nil {
		return nil, f.defaultTokenErr
	}
	return &services.TwitchTokenResponse{}, nil
}

func (f *fakeTwitchService) RefreshToken(_ context.Context, token string) (*services.TwitchRefreshTokenResponse, error) {
	f.refreshTokenCalls++
	f.lastRefreshToken = token
	if f.refreshTokenErr != nil {
		return nil, f.refreshTokenErr
	}
	return &services.TwitchRefreshTokenResponse{}, nil
}

func TestBuildRedirectURIPreservesDoubleSlashForCustomScheme(t *testing.T) {
	got, err := buildRedirectURI("foam://", "https://proxy.example/api/proxy?code=123")
	if err != nil {
		t.Fatalf("buildRedirectURI() error = %v", err)
	}

	if got != "foam://?code=123" {
		t.Fatalf("buildRedirectURI() = %q, want %q", got, "foam://?code=123")
	}
}

func TestRouteProxyMissingAppReturnsBadRequest(t *testing.T) {
	handlers := NewHandlers(&config.Proxy{
		Apps: map[string]config.AppConfig{
			"foam-app": {RedirectURI: "foam://"},
		},
	}, nil)

	status, _, body := handlers.Route(context.Background(), "/api/proxy", "https://proxy.example/api/proxy?code=123", map[string]string{
		"code": "123",
	})

	if status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}

	if !strings.Contains(body, "app query param is required") {
		t.Fatalf("body = %q", body)
	}
}

func TestRouteProxyUnknownAppReturnsBadRequest(t *testing.T) {
	handlers := NewHandlers(&config.Proxy{
		Apps: map[string]config.AppConfig{
			"foam-app": {RedirectURI: "foam://"},
		},
	}, nil)

	status, _, body := handlers.Route(context.Background(), "/api/proxy", "https://proxy.example/api/proxy?app=other&code=123", map[string]string{
		"app":  "other",
		"code": "123",
	})

	if status != 400 {
		t.Fatalf("status = %d, want 400", status)
	}

	if !strings.Contains(body, "unknown app") {
		t.Fatalf("body = %q", body)
	}
}

func TestTokenUsesSelectedAppService(t *testing.T) {
	appService := &fakeTwitchService{}
	menubarService := &fakeTwitchService{}

	handlers := NewHandlers(&config.Proxy{}, map[string]tokenService{
		"foam-app":     appService,
		"foam-menubar": menubarService,
	})

	_, err := handlers.Token(context.Background(), "foam-menubar")
	if err != nil {
		t.Fatalf("Token() error = %v", err)
	}

	if appService.defaultTokenCalls != 0 {
		t.Fatalf("foam-app service called %d times, want 0", appService.defaultTokenCalls)
	}
	if menubarService.defaultTokenCalls != 1 {
		t.Fatalf("foam-menubar service called %d times, want 1", menubarService.defaultTokenCalls)
	}
}

func TestRefreshTokenUsesSelectedAppService(t *testing.T) {
	appService := &fakeTwitchService{}
	menubarService := &fakeTwitchService{}

	handlers := NewHandlers(&config.Proxy{}, map[string]tokenService{
		"foam-app":     appService,
		"foam-menubar": menubarService,
	})

	_, err := handlers.RefreshToken(context.Background(), "foam-app", "refresh-123")
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	if appService.refreshTokenCalls != 1 || appService.lastRefreshToken != "refresh-123" {
		t.Fatalf("foam-app refresh calls = %d token = %q", appService.refreshTokenCalls, appService.lastRefreshToken)
	}
	if menubarService.refreshTokenCalls != 0 {
		t.Fatalf("foam-menubar service called %d times, want 0", menubarService.refreshTokenCalls)
	}
}

func TestTokenReturnsUnderlyingServiceError(t *testing.T) {
	handlers := NewHandlers(&config.Proxy{}, map[string]tokenService{
		"foam-app": &fakeTwitchService{defaultTokenErr: errors.New("boom")},
	})

	body, err := handlers.Token(context.Background(), "foam-app")
	if err == nil {
		t.Fatal("Token() error = nil, want error")
	}

	if !strings.Contains(body, "boom") {
		t.Fatalf("body = %q", body)
	}
}
