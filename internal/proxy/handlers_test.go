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
			bodyParts:   []string{"Foam - Redirecting", "Open Foam"},
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
