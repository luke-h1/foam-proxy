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

const twitchIdUrl = "https://id.twitch.tv/oauth2/token"

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
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

func NewTwitchService(clientID, clientSecret string, timeout time.Duration) *TwitchService {
	return &TwitchService{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: http.DefaultClient.Timeout},
	}
}

// request a client_credentials token from Twitch
func (s *TwitchService) DefaultToken() (*TwitchTokenResponse, error) {
	meter := sentry.NewMeter(context.Background())
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

	req, err := http.NewRequest(http.MethodPost, twitchIdUrl, bytes.NewBufferString(body))
	if err != nil {
		recordTokenMetric("error", "request_build", -1)
		captureTwitchError("token", err, "request_build", -1, 0, "")
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())

	if err != nil {
		recordTokenMetric("error", "request_failed", latencyMs)
		captureTwitchError("token", err, "request_failed", latencyMs, 0, "")
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		recordTokenMetric("error", "bad_status", latencyMs)
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		captureTwitchError("token", upstreamErr, "bad_status", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, upstreamErr
	}

	var out TwitchTokenResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		recordTokenMetric("error", "decode_failed", latencyMs)
		captureTwitchError("token", err, "decode_failed", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, err
	}

	recordTokenMetric("success", "", latencyMs)
	return &out, nil
}

func (s *TwitchService) RefreshToken(token string) (*TwitchRefreshTokenResponse, error) {
	meter := sentry.NewMeter(context.Background())

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

	req, err := http.NewRequest(http.MethodPost, twitchIdUrl, bytes.NewBufferString(body))
	if err != nil {
		recordRefreshTokenMetric("error", "request_build", -1)
		captureTwitchError("refresh_token", err, "request_build", -1, 0, "")
		return nil, err
	}

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())

	if err != nil {
		recordRefreshTokenMetric("error", "request_failed", latencyMs)
		captureTwitchError("refresh_token", err, "request_failed", latencyMs, 0, "")
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		recordRefreshTokenMetric("error", "bad_status", latencyMs)
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		captureTwitchError("refresh_token", upstreamErr, "bad_status", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, upstreamErr
	}

	var out TwitchRefreshTokenResponse
	if err := json.Unmarshal(bodyBytes, &out); err != nil {
		recordRefreshTokenMetric("error", "decode_failed", latencyMs)
		captureTwitchError("refresh_token", err, "decode_failed", latencyMs, resp.StatusCode, string(bodyBytes))
		return nil, err
	}

	recordRefreshTokenMetric("success", "", latencyMs)
	return &out, nil
}
