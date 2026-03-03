package proxy

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type Handlers struct {
	config *config.Proxy
	twitch *services.TwitchService
}

func NewHandlers(cfg *config.Proxy, twitch *services.TwitchService) *Handlers {
	return &Handlers{config: cfg, twitch: twitch}
}

func (handlers *Handlers) Health() string {
	meter := sentry.NewMeter(context.Background())
	meter.Count("health.check", 1)

	body, _ := json.Marshal(map[string]string{
		"status": "OK",
	})
	return string(body)
}

func (handlers *Handlers) Pending() string {
	return `<!DOCTYPE html>
	<html>
	  <head><title>Foam - Pending</title></head>
	  <body>
		<h1>Your request is pending</h1>
		<p>Please wait while we process your request.</p>
	  </body>
	</html>`
}

func (h *Handlers) Proxy() string {
	body, _ := json.Marshal(map[string]string{"message": "redirecting to app"})
	return string(body)
}

func (handlers *Handlers) Token() string {
	data, err := handlers.twitch.DefaultToken()

	if err != nil {
		body, _ := json.Marshal(map[string]interface{}{
			"data":  nil,
			"error": err.Error(),
		})
		return string(body)
	}

	body, _ := json.Marshal(map[string]interface{}{"data": data, "error": nil})
	return string(body)
}

func (handlers *Handlers) RefreshToken(token string) string {
	if token == "" {
		body, _ := json.Marshal(map[string]interface{}{
			"data":  nil,
			"error": "token query param is required",
		})
		return string(body)
	}

	data, err := handlers.twitch.RefreshToken(token)
	if err != nil {
		body, _ := json.Marshal(map[string]interface{}{
			"data":  nil,
			"error": err.Error(),
		})
		return string(body)
	}

	body, _ := json.Marshal(map[string]interface{}{"data": data, "error": nil})
	return string(body)
}

func (handlers *Handlers) Version() string {
	out := map[string]string{
		"deployedBy": "unknown",
		"deployedAt": "unknown",
		"gitSHA":     "unknown",
	}

	if handlers.config != nil {
		out["deployedBy"] = handlers.config.DeployedBy
		out["deployedAt"] = handlers.config.DeployedAt
		out["gitSHA"] = handlers.config.GitSHA
	}

	body, _ := json.Marshal(out)
	return string(body)
}

// redirects to the app with any query params
func RedirectURI(requestURL string) (string, error) {
	u, err := url.Parse(requestURL)
	if err != nil {
		return "", err
	}
	return "foam://?" + u.RawQuery, nil
}
