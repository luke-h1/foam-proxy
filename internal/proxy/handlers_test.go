package proxy

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

func TestRequestsHandle(t *testing.T) {
	proxyRequests := NewRequests(&config.Proxy{
		DeployedBy: "tester",
		DeployedAt: "now",
		GitSHA:     "abc123",
	}, nil)

	tests := []struct {
		name        string
		request     *events.APIGatewayProxyRequest
		statusCode  int
		contentType string
		bodyParts   []string
	}{
		{
			name:        "pending route returns html",
			request:     &events.APIGatewayProxyRequest{Path: "/api/pending"},
			statusCode:  200,
			contentType: "text/html",
			bodyParts:   []string{"Foam - Pending", "Open Foam"},
		},
		{
			name:        "proxy route returns html",
			request:     &events.APIGatewayProxyRequest{Path: "/api/proxy"},
			statusCode:  200,
			contentType: "text/html",
			bodyParts:   []string{"Foam - Redirecting", "Open Foam", "foam://"},
		},
		{
			name:        "health route returns json",
			request:     &events.APIGatewayProxyRequest{Path: "/api/healthcheck"},
			statusCode:  200,
			contentType: "application/json",
			bodyParts:   []string{`"status":"OK"`},
		},
		{
			name:        "version route returns json",
			request:     &events.APIGatewayProxyRequest{Path: "/api/version"},
			statusCode:  200,
			contentType: "application/json",
			bodyParts:   []string{`"deployedBy":"tester"`, `"gitSHA":"abc123"`},
		},
		{
			name:        "refresh token requires query parameter",
			request:     &events.APIGatewayProxyRequest{Path: "/api/refresh-token"},
			statusCode:  400,
			contentType: "application/json",
			bodyParts:   []string{`"error":"token query param is required"`},
		},
		{
			name:        "magic route is hidden when unconfigured",
			request:     &events.APIGatewayProxyRequest{Path: "/api/magic", QueryStringParameters: map[string]string{"key": "anything"}},
			statusCode:  404,
			contentType: "application/json",
			bodyParts:   []string{`"error":"not found"`},
		},
		{
			name:        "unknown path returns not found",
			request:     &events.APIGatewayProxyRequest{Path: "/api/unknown"},
			statusCode:  404,
			contentType: "application/json",
			bodyParts:   []string{`"error":"not found"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := proxyRequests.Handle(context.Background(), tt.request)
			if response.StatusCode != tt.statusCode {
				t.Fatalf("status code = %d, want %d", response.StatusCode, tt.statusCode)
			}
			if response.Headers["Content-Type"] != tt.contentType {
				t.Fatalf("content type = %q, want %q", response.Headers["Content-Type"], tt.contentType)
			}
			for _, bodyPart := range tt.bodyParts {
				if !strings.Contains(response.Body, bodyPart) {
					t.Fatalf("body %q does not contain %q", response.Body, bodyPart)
				}
			}
		})
	}
}

func TestRequestsHandleMagic(t *testing.T) {
	magic := &config.MagicLink{
		AccessToken:  "ABC123",
		RefreshToken: "REF456",
		ExpiresIn:    14400,
		TokenType:    "bearer",
	}
	proxyRequests := NewRequests(&config.Proxy{
		MagicLink:       magic,
		MagicLinkAPIKey: "s3cret-review-key",
	}, nil)

	tests := []struct {
		name        string
		query       map[string]string
		statusCode  int
		contentType string
		wantParts   []string
		absentParts []string
	}{
		{
			name:        "correct key redirects into the app with the stored token",
			query:       map[string]string{"key": "s3cret-review-key"},
			statusCode:  200,
			contentType: "text/html",
			wantParts: []string{
				"foam://auth?",
				"access_token=ABC123",
				"refresh_token=REF456",
				"token_type=bearer",
				"expires_in=14400",
			},
			absentParts: []string{"s3cret-review-key"},
		},
		{
			name:        "scheme override redirects into the requested variant app",
			query:       map[string]string{"key": "s3cret-review-key", "scheme": "foam-internal"},
			statusCode:  200,
			contentType: "text/html",
			wantParts: []string{
				"foam-internal://auth?",
				"access_token=ABC123",
			},
			absentParts: []string{"foam://auth?", "s3cret-review-key"},
		},
		{
			name:        "unknown scheme override falls back to the production scheme",
			query:       map[string]string{"key": "s3cret-review-key", "scheme": "https://evil.example"},
			statusCode:  200,
			contentType: "text/html",
			wantParts: []string{
				"foam://auth?",
				"access_token=ABC123",
			},
			absentParts: []string{"evil.example", "s3cret-review-key"},
		},
		{
			name:        "correct key with format=json returns the raw session blob",
			query:       map[string]string{"key": "s3cret-review-key", "format": "json"},
			statusCode:  200,
			contentType: "application/json",
			wantParts: []string{
				`"access_token":"ABC123"`,
				`"refresh_token":"REF456"`,
				`"token_type":"bearer"`,
				`"expires_in":14400`,
			},
			absentParts: []string{"foam://auth", "s3cret-review-key"},
		},
		{
			name:        "wrong key with format=json is still indistinguishable from a missing route",
			query:       map[string]string{"key": "wrong", "format": "json"},
			statusCode:  404,
			contentType: "application/json",
			wantParts:   []string{`"error":"not found"`},
		},
		{
			name:        "wrong key is indistinguishable from a missing route",
			query:       map[string]string{"key": "wrong"},
			statusCode:  404,
			contentType: "application/json",
			wantParts:   []string{`"error":"not found"`},
		},
		{
			name:        "missing key returns not found",
			query:       nil,
			statusCode:  404,
			contentType: "application/json",
			wantParts:   []string{`"error":"not found"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := proxyRequests.Handle(context.Background(), &events.APIGatewayProxyRequest{
				Path:                  "/api/magic",
				QueryStringParameters: tt.query,
			})

			if response.StatusCode != tt.statusCode {
				t.Fatalf("status code = %d, want %d", response.StatusCode, tt.statusCode)
			}
			if response.Headers["Content-Type"] != tt.contentType {
				t.Fatalf("content type = %q, want %q", response.Headers["Content-Type"], tt.contentType)
			}
			for _, part := range tt.wantParts {
				if !strings.Contains(response.Body, part) {
					t.Fatalf("body does not contain %q\nbody: %s", part, response.Body)
				}
			}
			for _, part := range tt.absentParts {
				if strings.Contains(response.Body, part) {
					t.Fatalf("body leaked %q\nbody: %s", part, response.Body)
				}
			}
		})
	}
}

type stubTwitch struct {
	err error
}

func (s stubTwitch) DefaultToken(context.Context) (*services.TwitchTokenResponse, error) {
	return nil, s.err
}

func (s stubTwitch) RefreshToken(context.Context, string) (*services.TwitchRefreshTokenResponse, error) {
	return nil, s.err
}

func TestHandleTokenAndRefreshSurfaceUpstreamFailure(t *testing.T) {
	proxyRequests := NewRequests(&config.Proxy{}, nil)
	proxyRequests.twitch = stubTwitch{err: errors.New("upstream down")}

	token := proxyRequests.Handle(context.Background(), &events.APIGatewayProxyRequest{Path: "/api/token"})
	if token.StatusCode != http.StatusBadGateway || !strings.Contains(token.Body, "upstream down") {
		t.Fatalf("token response = %d %s, want 502 upstream down", token.StatusCode, token.Body)
	}

	refresh := proxyRequests.Handle(context.Background(), &events.APIGatewayProxyRequest{
		Path:                  "/api/refresh-token",
		QueryStringParameters: map[string]string{"token": "abc"},
	})
	if refresh.StatusCode != http.StatusBadGateway || !strings.Contains(refresh.Body, "upstream down") {
		t.Fatalf("refresh response = %d %s, want 502 upstream down", refresh.StatusCode, refresh.Body)
	}
}

func TestBuildRequestURLRedactsSecrets(t *testing.T) {
	got := buildRequestURL(&events.APIGatewayProxyRequest{
		Path:    "/api/refresh-token",
		Headers: map[string]string{"Host": "auth.example"},
		QueryStringParameters: map[string]string{
			"token": "super-secret-token",
			"key":   "magic-link-key",
		},
	})
	if strings.Contains(got, "super-secret-token") || strings.Contains(got, "magic-link-key") {
		t.Fatalf("url leaked secrets: %s", got)
	}

	short := buildRequestURL(&events.APIGatewayProxyRequest{
		Path: "/api/magic",
		QueryStringParameters: map[string]string{
			"key": "shortkey",
		},
	})
	if strings.Contains(short, "shortkey") || !strings.Contains(short, "key=%5Bredacted%5D") && !strings.Contains(short, "key=[redacted]") {
		t.Fatalf("url leaked short key: %s", short)
	}
}
