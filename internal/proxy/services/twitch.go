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

	"github.com/foam/proxy/internal/observability"
)

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
	runtime      twitchRuntime
}

type twitchRuntime interface {
	RecordTwitchSuccess(ctx context.Context, operation, app string, statusCode int, duration time.Duration)
	RecordTwitchFailure(ctx context.Context, operation, app, reason string, statusCode int, duration time.Duration, responseBody string, err error)
}

type twitchExchangeSpec struct {
	operation      string
	metricName     string
	latencyMetric  string
	grantType      string
	extraForm      url.Values
	decodeResponse func(body []byte) error
}

func NewTwitchService(app, clientID, clientSecret string, timeout time.Duration, runtime twitchRuntime) *TwitchService {
	return &TwitchService{
		app:          app,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient:   &http.Client{Timeout: timeout},
		runtime:      runtime,
	}
}

// request a client_credentials token from Twitch
func (s *TwitchService) DefaultToken(ctx context.Context) (*TwitchTokenResponse, error) {
	var out TwitchTokenResponse
	err := s.exchange(ctx, twitchExchangeSpec{
		operation:     "default_token",
		metricName:    "twitch.token.request",
		latencyMetric: "twitch.token.latency_ms",
		grantType:     "client_credentials",
		decodeResponse: func(body []byte) error {
			return json.Unmarshal(body, &out)
		},
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *TwitchService) RefreshToken(ctx context.Context, token string) (*TwitchRefreshTokenResponse, error) {
	var out TwitchRefreshTokenResponse
	err := s.exchange(ctx, twitchExchangeSpec{
		operation:     "refresh_token",
		metricName:    "twitch.refresh_token.request",
		latencyMetric: "twitch.refresh_token.latency_ms",
		grantType:     "refresh_token",
		extraForm: url.Values{
			"refresh_token": []string{token},
		},
		decodeResponse: func(body []byte) error {
			return json.Unmarshal(body, &out)
		},
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *TwitchService) recordFailure(ctx context.Context, operation, reason string, statusCode int, duration time.Duration, responseBody string, err error) {
	if s.runtime == nil {
		return
	}
	s.runtime.RecordTwitchFailure(ctx, operation, s.app, reason, statusCode, duration, responseBody, err)
}

func (s *TwitchService) recordSuccess(ctx context.Context, operation string, statusCode int, duration time.Duration) {
	if s.runtime == nil {
		return
	}
	s.runtime.RecordTwitchSuccess(ctx, operation, s.app, statusCode, duration)
}

func (s *TwitchService) exchange(ctx context.Context, spec twitchExchangeSpec) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchIdURL, bytes.NewBufferString(s.exchangeForm(spec).Encode()))
	if err != nil {
		s.recordFailure(ctx, spec.operation, "request_build", 0, -1, "", err)
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		s.recordFailure(ctx, spec.operation, "request_failed", 0, latency, "", err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		upstreamErr := twitchErrorFromResponse(resp.StatusCode, bodyBytes)
		s.recordFailure(ctx, spec.operation, "bad_status", resp.StatusCode, latency, string(bodyBytes), upstreamErr)
		return upstreamErr
	}

	if err := spec.decodeResponse(bodyBytes); err != nil {
		s.recordFailure(ctx, spec.operation, "decode_failed", resp.StatusCode, latency, string(bodyBytes), err)
		return err
	}

	s.recordSuccess(ctx, spec.operation, resp.StatusCode, latency)
	return nil
}

func (s *TwitchService) exchangeForm(spec twitchExchangeSpec) url.Values {
	form := url.Values{}
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)
	form.Set("grant_type", spec.grantType)
	for key, values := range spec.extraForm {
		for _, value := range values {
			form.Add(key, value)
		}
	}
	return form
}

var _ twitchRuntime = (*observability.Runtime)(nil)
