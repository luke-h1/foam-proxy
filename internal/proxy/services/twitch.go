package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

func captureTwitchError(ctx context.Context, operation string, err error, reason string, latencyMs float64, statusCode int, responseBody string) {
	if err == nil {
		return
	}
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.WithScope(func(scope *sentry.Scope) {
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
		hub.CaptureException(err)
	})
}

type twitchErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func twitchErrorFromResponse(statusCode int, body []byte) error {
	var twErr twitchErrorResponse
	if json.Unmarshal(body, &twErr) == nil && twErr.Message != "" {
		return errors.New(twErr.Message)
	}
	return fmt.Errorf("twitch error: %d %s", statusCode, string(bytes.TrimSpace(body)))
}

const (
	twitchIDURL    = "https://id.twitch.tv/oauth2/token"
	defaultTimeout = 8 * time.Second
)

type TwitchTokenResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	TokenType   string   `json:"token_type"`
	Scope       []string `json:"scope"`
}

type TwitchRefreshTokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	Scope        []string `json:"scope"`
	TokenType    string   `json:"token_type"`
}

type TwitchService struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client
}

func NewTwitchService(clientID, clientSecret string, timeout time.Duration) *TwitchService {
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &TwitchService{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      twitchIDURL,
		httpClient: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          10,
				MaxIdleConnsPerHost:   2,
				IdleConnTimeout:       60 * time.Second,
				TLSHandshakeTimeout:   2 * time.Second,
				ResponseHeaderTimeout: timeout,
				ExpectContinueTimeout: 1 * time.Second,
			},
		},
	}
}

// request a client_credentials token from Twitch.
func (s *TwitchService) DefaultToken(ctx context.Context) (*TwitchTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", "client_credentials")

	var out TwitchTokenResponse
	if err := s.postForm(ctx, "token", "twitch.token", form, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *TwitchService) RefreshToken(ctx context.Context, token string) (*TwitchRefreshTokenResponse, error) {
	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", token)

	var out TwitchRefreshTokenResponse
	if err := s.postForm(ctx, "refresh_token", "twitch.refresh_token", form, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *TwitchService) postForm(ctx context.Context, operation, metricPrefix string, form url.Values, dest any) error {
	meter := sentry.NewMeter(ctx)
	record := func(outcome, reason string, latencyMs float64) {
		attrs := []attribute.Builder{attribute.String("outcome", outcome)}
		if reason != "" {
			attrs = append(attrs, attribute.String("reason", reason))
		}
		meter.Count(metricPrefix+".request", 1, sentry.WithAttributes(attrs...))
		if latencyMs >= 0 {
			meter.Distribution(metricPrefix+".latency_ms", latencyMs, sentry.WithUnit(sentry.UnitMillisecond))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL, strings.NewReader(form.Encode()))
	if err != nil {
		err = fmt.Errorf("twitch: build request: %w", err)
		record("error", "request_build", -1)
		captureTwitchError(ctx, operation, err, "request_build", -1, 0, "")
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())
	if err != nil {
		err = fmt.Errorf("twitch: do request: %w", err)
		record("error", "request_failed", latencyMs)
		captureTwitchError(ctx, operation, err, "request_failed", latencyMs, 0, "")
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			slog.Warn("close twitch response body", "err", closeErr)
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("twitch: read response: %w", err)
		record("error", "read_failed", latencyMs)
		captureTwitchError(ctx, operation, err, "read_failed", latencyMs, resp.StatusCode, "")
		return err
	}
	if resp.StatusCode != http.StatusOK {
		record("error", "bad_status", latencyMs)
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		captureTwitchError(ctx, operation, upstreamErr, "bad_status", latencyMs, resp.StatusCode, string(bodyBytes))
		return upstreamErr
	}
	if err := json.Unmarshal(bodyBytes, dest); err != nil {
		err = fmt.Errorf("twitch: decode response: %w", err)
		record("error", "decode_failed", latencyMs)
		captureTwitchError(ctx, operation, err, "decode_failed", latencyMs, resp.StatusCode, "")
		return err
	}

	record("success", "", latencyMs)
	return nil
}
