package proxy

import (
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
)

func TestProxyRequestsHandle(t *testing.T) {
	proxyRequests := NewProxyRequests(&config.Proxy{
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
			response := proxyRequests.Handle(tt.request)
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

func TestProxyRequestsHandleMagic(t *testing.T) {
	magic := &config.MagicLink{
		AccessToken:  "ABC123",
		RefreshToken: "REF456",
		ExpiresIn:    14400,
		TokenType:    "bearer",
	}
	proxyRequests := NewProxyRequests(&config.Proxy{
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
			response := proxyRequests.Handle(&events.APIGatewayProxyRequest{
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
