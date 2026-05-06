package proxy

import (
	"context"
	"encoding/json"
	"fmt"

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
	return redirectPage("Foam - Pending", "foam://")
}

func (h *Handlers) Proxy() string {
	return redirectPage("Foam - Redirecting", "foam://")
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

func redirectPage(title, targetPrefix string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <h1>Redirecting…</h1>
    <p>If nothing happens automatically, return to Foam.</p>
    <script>
      const search = window.location.search.replace(/^\?/, '');
      const hash = window.location.hash.replace(/^#/, '');
      const params = new URLSearchParams(search);
      const hashParams = new URLSearchParams(hash);

      for (const [key, value] of hashParams.entries()) {
        params.set(key, value);
      }

      const query = params.toString();
      window.location.replace(query ? '%s?' + query : '%s');
    </script>
  </body>
</html>`, title, targetPrefix, targetPrefix)
}
