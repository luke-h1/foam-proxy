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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"
)

type pushContextPusher interface {
	PushContext(ctx context.Context) error
}

type Metrics struct {
	registry              *prometheus.Registry
	pusher                pushContextPusher
	pushMu                sync.Mutex
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
	twitchRequestsTotal   *prometheus.CounterVec
	twitchRequestDuration *prometheus.HistogramVec
	configInfo            *prometheus.GaugeVec
}

func Init(serviceName string, apps []string) (*Metrics, error) {
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

	m := &Metrics{
		registry:              registry,
		requestsTotal:         requestsTotal,
		requestDuration:       requestDuration,
		twitchRequestsTotal:   twitchRequestsTotal,
		twitchRequestDuration: twitchRequestDuration,
		configInfo:            configInfo,
	}

	gatewayURL := normalizePushgatewayURL(os.Getenv("PUSHGATEWAY_URL"))
	if gatewayURL == "" {
		return m, nil
	}

	pusher := push.New(gatewayURL, serviceName).
		Grouping("instance", pushgatewayInstance()).
		Format(expfmt.NewFormat(expfmt.TypeTextPlain)).
		Gatherer(registry)

	if headers := pushgatewayHeadersFromEnv(); len(headers) > 0 {
		pusher = pusher.Header(headers)
	}

	if err := pusher.Error(); err != nil {
		return nil, err
	}
	m.pusher = pusher

	return m, nil
}

func (m *Metrics) Push(ctx context.Context) error {
	if m == nil || m.pusher == nil {
		return nil
	}

	m.pushMu.Lock()
	defer m.pushMu.Unlock()

	return m.pusher.PushContext(ctx)
}

func (m *Metrics) PushWithTimeout(timeout time.Duration) error {
	if m == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.Push(ctx)
}

func (m *Metrics) RecordRequest(_ context.Context, route, app string, statusCode int, duration time.Duration) {
	if m == nil {
		return
	}

	labels := []string{route, normalizeApp(app), statusClass(statusCode)}
	m.requestsTotal.WithLabelValues(labels...).Inc()
	m.requestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
}

func normalizeApp(app string) string {
	app = strings.TrimSpace(app)
	if app == "" {
		return "unknown"
	}
	return app
}

func (m *Metrics) RecordTwitchRequest(_ context.Context, operation, app, outcome string, statusCode int, duration time.Duration) {
	if m == nil {
		return
	}

	labels := []string{operation, normalizeApp(app), outcome, statusClass(statusCode)}
	m.twitchRequestsTotal.WithLabelValues(labels...).Inc()
	m.twitchRequestDuration.WithLabelValues(labels...).Observe(duration.Seconds())
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
