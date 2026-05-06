package observability

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/common/expfmt"
)

func TestMetricsPushesRecordedRequestAndConfigInfo(t *testing.T) {
	t.Setenv("GIT_SHA", "abc123")
	t.Setenv("SENTRY_ENVIRONMENT", "staging")

	metrics := NewRuntime("foam-proxy", []string{"foam-app", "foam-menubar"})
	pusher := &fakePusher{}
	metrics.pusher = pusher

	metrics.RecordTwitchSuccess(context.Background(), "default_token", "foam-app", 200, 25*time.Millisecond)
	metrics.RecordRequest(context.Background(), "/api/version", "foam-app", 200, 50*time.Millisecond)

	if err := metrics.Push(context.Background()); err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if pusher.calls != 1 {
		t.Fatalf("pusher.calls = %d, want 1", pusher.calls)
	}

	var body bytes.Buffer
	metricFamilies, err := metrics.registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	for _, family := range metricFamilies {
		if _, err := expfmt.MetricFamilyToText(&body, family); err != nil {
			t.Fatalf("MetricFamilyToText() error = %v", err)
		}
	}
	requestBody := body.String()

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

type fakePusher struct {
	calls int
}

func (f *fakePusher) PushContext(context.Context) error {
	f.calls++
	return nil
}
