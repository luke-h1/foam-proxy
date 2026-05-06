package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"
)

type ProxyRuntime interface {
	RecordRequest(ctx context.Context, route, app string, statusCode int, duration time.Duration)
	RecordTwitchSuccess(ctx context.Context, operation, app string, statusCode int, duration time.Duration)
	RecordTwitchFailure(ctx context.Context, operation, app, reason string, statusCode int, duration time.Duration, responseBody string, err error)
	RecordHealthCheck(ctx context.Context)
	Push(ctx context.Context) error
}

type AuthorizerRuntime interface {
	RecordAuthorization(ctx context.Context, outcome, reason string)
}

type pushContextPusher interface {
	PushContext(ctx context.Context) error
}

type Runtime struct {
	registry              *prometheus.Registry
	pusher                pushContextPusher
	pushMu                sync.Mutex
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
	twitchRequestsTotal   *prometheus.CounterVec
	twitchRequestDuration *prometheus.HistogramVec
	configInfo            *prometheus.GaugeVec
}

func NewRuntime(serviceName string, apps []string) *Runtime {
	registry := prometheus.NewRegistry()

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "foam_proxy_requests_total",
			Help: "Total inbound proxy requests",
		},
		[]string{"route", "app", "status_code_class"},
	)
	registry.MustRegister(requestsTotal)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "foam_proxy_request_duration_seconds",
			Help:    "Inbound proxy request latency",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"route", "app", "status_code_class"},
	)
	registry.MustRegister(requestDuration)

	twitchRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "foam_proxy_twitch_requests_total",
			Help: "Total Twitch upstream requests",
		},
		[]string{"operation", "app", "outcome", "status_code_class"},
	)
	registry.MustRegister(twitchRequestsTotal)

	twitchRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "foam_proxy_twitch_request_duration_seconds",
			Help:    "Twitch upstream request latency",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation", "app", "outcome", "status_code_class"},
	)
	registry.MustRegister(twitchRequestDuration)

	configInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "foam_proxy_config_info",
			Help: "Build and config metadata",
		},
		[]string{"app", "git_sha", "environment"},
	)
	registry.MustRegister(configInfo)

	environment := envOrUnknown("SENTRY_ENVIRONMENT")
	gitSHA := envOrUnknown("GIT_SHA")
	for _, app := range configuredApps(apps) {
		configInfo.WithLabelValues(app, gitSHA, environment).Set(1)
	}

	runtime := &Runtime{
		registry:              registry,
		requestsTotal:         requestsTotal,
		requestDuration:       requestDuration,
		twitchRequestsTotal:   twitchRequestsTotal,
		twitchRequestDuration: twitchRequestDuration,
		configInfo:            configInfo,
	}

	gatewayURL := normalizePushgatewayURL(os.Getenv("PUSHGATEWAY_URL"))
	if gatewayURL == "" {
		return runtime
	}

	pusher := push.New(gatewayURL, serviceName).
		Grouping("instance", pushgatewayInstance()).
		Format(expfmt.NewFormat(expfmt.TypeTextPlain)).
		Gatherer(registry)

	if headers := pushgatewayHeadersFromEnv(); len(headers) > 0 {
		pusher = pusher.Header(headers)
	}

	if err := pusher.Error(); err != nil {
		return runtime
	}
	runtime.pusher = pusher

	return runtime
}

func (r *Runtime) Push(ctx context.Context) error {
	if r == nil || r.pusher == nil {
		return nil
	}

	r.pushMu.Lock()
	defer r.pushMu.Unlock()

	return r.pusher.PushContext(ctx)
}

func (r *Runtime) RecordRequest(_ context.Context, route, app string, statusCode int, duration time.Duration) {
	if r == nil {
		return
	}

	labels := []string{route, normalizeApp(app), statusClass(statusCode)}
	r.requestsTotal.WithLabelValues(labels...).Inc()
	r.requestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
}

func normalizeApp(app string) string {
	app = strings.TrimSpace(app)
	if app == "" {
		return "unknown"
	}
	return app
}

func (r *Runtime) RecordTwitchSuccess(ctx context.Context, operation, app string, statusCode int, duration time.Duration) {
	r.recordTwitchMetrics(operation, app, "success", statusCode, duration)
	meter := sentry.NewMeter(ctx)
	meter.Count(twitchCounterName(operation), 1, sentry.WithAttributes(attribute.String("outcome", "success")))
	meter.Distribution(twitchLatencyName(operation), float64(duration.Milliseconds()), sentry.WithUnit(sentry.UnitMillisecond))
}

func (r *Runtime) RecordTwitchFailure(ctx context.Context, operation, app, reason string, statusCode int, duration time.Duration, responseBody string, err error) {
	r.recordTwitchMetrics(operation, app, "error", statusCode, duration)

	attrs := []attribute.Builder{attribute.String("outcome", "error")}
	if reason != "" {
		attrs = append(attrs, attribute.String("reason", reason))
	}
	meter := sentry.NewMeter(ctx)
	meter.Count(twitchCounterName(operation), 1, sentry.WithAttributes(attrs...))
	if duration >= 0 {
		meter.Distribution(twitchLatencyName(operation), float64(duration.Milliseconds()), sentry.WithUnit(sentry.UnitMillisecond))
	}

	if err == nil {
		return
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("twitch.operation", operation)
		scope.SetTag("twitch.error_reason", reason)
		scope.SetLevel(sentry.LevelError)
		if duration >= 0 {
			scope.SetExtra("latency_ms", float64(duration.Milliseconds()))
		}
		if statusCode > 0 {
			scope.SetExtra("http_status", statusCode)
		}
		if responseBody != "" {
			scope.SetExtra("twitch.response_body", responseBody)
		}
		sentry.CaptureException(err)
	})
}

func (r *Runtime) RecordHealthCheck(ctx context.Context) {
	meter := sentry.NewMeter(ctx)
	meter.Count("health.check", 1)
}

func (r *Runtime) RecordAuthorization(ctx context.Context, outcome, reason string) {
	meter := sentry.NewMeter(ctx)
	attrs := []attribute.Builder{attribute.String("outcome", outcome)}
	if reason != "" {
		attrs = append(attrs, attribute.String("reason", reason))
	}
	meter.Count("authorizer.authorization", 1, sentry.WithAttributes(attrs...))
}

func (r *Runtime) recordTwitchMetrics(operation, app, outcome string, statusCode int, duration time.Duration) {
	if r == nil {
		return
	}

	labels := []string{operation, normalizeApp(app), outcome, statusClass(statusCode)}
	r.twitchRequestsTotal.WithLabelValues(labels...).Inc()
	r.twitchRequestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
}

func statusClass(code int) string {
	if code < 100 {
		return "unknown"
	}
	return fmt.Sprintf("%dxx", code/100)
}

func normalizePushgatewayURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func pushgatewayHeadersFromEnv() http.Header {
	raw := strings.TrimSpace(os.Getenv("PUSHGATEWAY_AUTH_HEADER"))
	if raw == "" {
		return nil
	}

	header := make(http.Header)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name := ""
		value := ""
		if k, v, ok := strings.Cut(part, "="); ok {
			name, value = strings.TrimSpace(k), strings.TrimSpace(v)
		} else if k, v, ok := strings.Cut(part, ":"); ok {
			name, value = strings.TrimSpace(k), strings.TrimSpace(v)
		} else if strings.HasPrefix(strings.ToLower(part), "basic ") {
			name, value = "Authorization", part
		}

		if name != "" && value != "" {
			header.Set(name, value)
		}
	}

	if len(header) == 0 {
		return nil
	}
	return header
}

func pushgatewayInstance() string {
	if logStream := strings.TrimSpace(os.Getenv("AWS_LAMBDA_LOG_STREAM_NAME")); logStream != "" {
		return logStream
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return "unknown"
}

func configuredApps(apps []string) []string {
	seen := make(map[string]struct{}, len(apps))
	out := make([]string, 0, len(apps))
	for _, app := range apps {
		app = normalizeApp(app)
		if _, ok := seen[app]; ok {
			continue
		}
		seen[app] = struct{}{}
		out = append(out, app)
	}
	sort.Strings(out)
	return out
}

func envOrUnknown(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "unknown"
	}
	return value
}

func twitchCounterName(operation string) string {
	switch operation {
	case "refresh_token":
		return "twitch.refresh_token.request"
	default:
		return "twitch.token.request"
	}
}

func twitchLatencyName(operation string) string {
	switch operation {
	case "refresh_token":
		return "twitch.refresh_token.latency_ms"
	default:
		return "twitch.token.latency_ms"
	}
}

var _ ProxyRuntime = (*Runtime)(nil)
var _ AuthorizerRuntime = (*Runtime)(nil)
