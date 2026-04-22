package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

func captureTwitchError(operation string, err error, reason string, latencyMs float64, statusCode int, responseBody string) {
	if err == nil {
		return
	}
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("twitch.operation", operation)
		scope.SetTag("twitch.error_reason", reason)
		scope.SetLevel(sentry.LevelError)
		if latencyMs >= 0 {
			scope.SetExtra("latency_ms", latencyMs)
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

type twitchErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func twitchErrorFromResponse(statusCode int, body []byte) error {
	var twErr twitchErrorResponse
	if json.Unmarshal(body, &twErr) == nil && twErr.Message != "" {
		return fmt.Errorf("%s", twErr.Message)
	}
	return fmt.Errorf("twitch error: %d %s", statusCode, string(bytes.TrimSpace(body)))
}

var twitchIdURL = "https://id.twitch.tv/oauth2/token"

type TwitchTokenResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	TokenType   string   `json:"token_type"`
	Scope       []string `json:"scope"`
}

type TwitchRefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type TwitchService struct {
	app          string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	metrics      twitchMetricsRecorder
}

type twitchMetricsRecorder interface {
	RecordTwitchRequest(ctx context.Context, operation, app, outcome string, statusCode int, duration time.Duration)
}

func NewTwitchService(app, clientID, clientSecret string, timeout time.Duration, metrics twitchMetricsRecorder) *TwitchService {
	return &TwitchService{
		app:          app,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: timeout},
		metrics:      metrics,
	}
}

// request a client_credentials token from Twitch
func (s *TwitchService) DefaultToken(ctx context.Context) (*TwitchTokenResponse, error) {
	meter := sentry.NewMeter(ctx)
	recordTokenMetric := func(outcome string, reason string, latencyMs float64) {
		attrs := []attribute.Builder{attribute.String("outcome", outcome)}
		if reason != "" {
			attrs = append(attrs, attribute.String("reason", reason))
		}
		meter.Count("twitch.token.request", 1, sentry.WithAttributes(attrs...))
		if latencyMs >= 0 {
			meter.Distribution("twitch.token.latency_ms", latencyMs, sentry.WithUnit(sentry.UnitMillisecond))
		}
	}

	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", "client_credentials")
	body := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchIdURL, bytes.NewBufferString(body))
	if err != nil {
		recordTokenMetric("error", "request_build", -1)
		captureTwitchError("token", err, "request_build", -1, 0, "")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start)
	latencyMs := float64(latency.Milliseconds())

	if err != nil {
		recordTokenMetric("error", "request_failed", latencyMs)
		s.recordMetrics(ctx, "default_token", "error", 0, latency)
		captureTwitchError("token", err, "request_failed", latencyMs, 0, "")
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		recordTokenMetric("error", "bad_status", latencyMs)
		s.recordMetrics(ctx, "default_token", "error", resp.StatusCode, latency)
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		captureTwitchError("token", upstreamErr, "bad_status", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, upstreamErr
	}

	var out TwitchTokenResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		recordTokenMetric("error", "decode_failed", latencyMs)
		s.recordMetrics(ctx, "default_token", "error", resp.StatusCode, latency)
		captureTwitchError("token", err, "decode_failed", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, err
	}

	recordTokenMetric("success", "", latencyMs)
	s.recordMetrics(ctx, "default_token", "success", resp.StatusCode, latency)
	return &out, nil
}

func (s *TwitchService) RefreshToken(ctx context.Context, token string) (*TwitchRefreshTokenResponse, error) {
	meter := sentry.NewMeter(ctx)

	recordRefreshTokenMetric := func(outcome string, reason string, latencyMs float64) {
		attrs := []attribute.Builder{attribute.String("outcome", outcome)}
		if reason != "" {
			attrs = append(attrs, attribute.String("reason", reason))
		}
		meter.Count("twitch.refresh_token.request", 1, sentry.WithAttributes(attrs...))
		if latencyMs >= 0 {
			meter.Distribution("twitch.refresh_token.latency_ms", latencyMs, sentry.WithUnit(sentry.UnitMillisecond))
		}
	}

	form := url.Values{}

	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", token)

	body := form.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchIdURL, bytes.NewBufferString(body))
	if err != nil {
		recordRefreshTokenMetric("error", "request_build", -1)
		captureTwitchError("refresh_token", err, "request_build", -1, 0, "")
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start)
	latencyMs := float64(latency.Milliseconds())

	if err != nil {
		recordRefreshTokenMetric("error", "request_failed", latencyMs)
		s.recordMetrics(ctx, "refresh_token", "error", 0, latency)
		captureTwitchError("refresh_token", err, "request_failed", latencyMs, 0, "")
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		recordRefreshTokenMetric("error", "bad_status", latencyMs)
		s.recordMetrics(ctx, "refresh_token", "error", resp.StatusCode, latency)
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		captureTwitchError("refresh_token", upstreamErr, "bad_status", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, upstreamErr
	}

	var out TwitchRefreshTokenResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		recordRefreshTokenMetric("error", "decode_failed", latencyMs)
		s.recordMetrics(ctx, "refresh_token", "error", resp.StatusCode, latency)
		captureTwitchError("refresh_token", err, "decode_failed", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, err
	}

	recordRefreshTokenMetric("success", "", latencyMs)
	s.recordMetrics(ctx, "refresh_token", "success", resp.StatusCode, latency)
	return &out, nil
}

func (s *TwitchService) recordMetrics(ctx context.Context, operation, outcome string, statusCode int, duration time.Duration) {
	if s.metrics == nil {
		return
	}
	s.metrics.RecordTwitchRequest(ctx, operation, s.app, outcome, statusCode, duration)
}
