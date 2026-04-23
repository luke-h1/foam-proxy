package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMetricsPushesRecordedRequestAndConfigInfo(t *testing.T) {
	t.Setenv("AWS_LAMBDA_LOG_STREAM_NAME", "stream-1")
	t.Setenv("GIT_SHA", "abc123")
	t.Setenv("SENTRY_ENVIRONMENT", "staging")

	var (
		mu          sync.Mutex
		requestPath string
		requestBody string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		mu.Lock()
		requestPath = r.URL.Path
		requestBody = string(body)
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	t.Setenv("PUSHGATEWAY_URL", server.URL)

	metrics, err := Init("foam-proxy", []string{"foam-app", "foam-menubar"})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	metrics.RecordTwitchRequest(context.Background(), "default_token", "foam-app", "success", http.StatusOK, 25*time.Millisecond)
	metrics.RecordRequest(context.Background(), "/api/version", "foam-app", http.StatusOK, 50*time.Millisecond)

	if err := metrics.Push(context.Background()); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if requestPath != "/metrics/job/foam-proxy/instance/stream-1" {
		t.Fatalf("push path = %q", requestPath)
	}

	for _, needle := range []string{
		`foam_proxy_requests_total{app="foam-app",route="/api/version",status_code_class="2xx"} 1`,
		`foam_proxy_twitch_requests_total{app="foam-app",operation="default_token",outcome="success",status_code_class="2xx"} 1`,
		`foam_proxy_config_info{app="foam-app",environment="staging",git_sha="abc123"} 1`,
		`foam_proxy_config_info{app="foam-menubar",environment="staging",git_sha="abc123"} 1`,
	} {
		if !strings.Contains(requestBody, needle) {
			t.Fatalf("push body missing %q\nbody:\n%s", needle, requestBody)
		}
	}
}

func TestMetricsUsesPushgatewayAuthHeader(t *testing.T) {
	t.Setenv("PUSHGATEWAY_AUTH_HEADER", "Authorization=Basic dGVzdDp0ZXN0")

	headers := pushgatewayHeadersFromEnv()
	if got := headers.Get("Authorization"); got != "Basic dGVzdDp0ZXN0" {
		t.Fatalf("Authorization header = %q", got)
	}
}

func TestNormalizePushgatewayURLTrimsTrailingSlash(t *testing.T) {
	got := normalizePushgatewayURL("https://pushgateway.example.com/")
	if got != "https://pushgateway.example.com" {
		t.Fatalf("normalizePushgatewayURL() = %q", got)
	}
}
