package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

const twitchIdUrl = "https://id.twitch.tv/oauth2/token"

type TwitchTokenResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	TokenType   string   `json:"token_type"`
	Scope       []string `json:"scope"`
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
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	latencyMs := float64(time.Since(start).Milliseconds())

	if err != nil {
		recordTokenMetric("error", "request_failed", latencyMs)
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	var out TwitchTokenResponse

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		recordTokenMetric("error", "decode_failed", latencyMs)
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		recordTokenMetric("error", "bad_status", latencyMs)
		return nil, fmt.Errorf("failed to get token: %s", resp.Status)
	}

	recordTokenMetric("success", "", latencyMs)
	return &out, nil
}
