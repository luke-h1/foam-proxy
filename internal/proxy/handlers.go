package proxy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type ProxyRequests struct {
	config *config.Proxy
	twitch *services.TwitchService
}

func NewProxyRequests(cfg *config.Proxy, twitch *services.TwitchService) *ProxyRequests {
	return &ProxyRequests{config: cfg, twitch: twitch}
}

func (p *ProxyRequests) Handle(req *events.APIGatewayProxyRequest) Response {
	if req == nil {
		return p.jsonResponse(500, map[string]string{"error": "invalid request"})
	}

	switch req.Path {
	case "/api/pending":
		return p.handlePending()
	case "/api/proxy":
		return p.handleProxy()
	case "/api/token":
		return p.handleToken()
	case "/api/refresh-token":
		return p.handleRefreshToken(req)
	case "/api/healthcheck":
		return p.handleHealth()
	case "/api/version":
		return p.handleVersion()
	default:
		return p.jsonResponse(404, map[string]string{"error": "not found"})
	}
}

func (p *ProxyRequests) handleHealth() Response {
	meter := sentry.NewMeter(context.Background())
	meter.Count("health.check", 1)

	return p.jsonResponse(200, map[string]string{
		"status": "OK",
	})
}

func (p *ProxyRequests) handlePending() Response {
	safeLog("[AUTHDBG] pending page served", map[string]string{
		"target": "foam://",
	})
	return htmlResponse(200, redirectPage("Foam - Pending", "foam://"))
}

func (p *ProxyRequests) handleProxy() Response {
	safeLog("[AUTHDBG] proxy page served", map[string]string{
		"target": "foam://",
	})
	return htmlResponse(200, redirectPage("Foam - Redirecting", "foam://"))
}

func (p *ProxyRequests) handleToken() Response {
	data, err := p.twitch.DefaultToken()

	if err != nil {
		return p.jsonResponse(200, map[string]interface{}{
			"data":  nil,
			"error": err.Error(),
		})
	}

	return p.jsonResponse(200, map[string]interface{}{"data": data, "error": nil})
}

func (p *ProxyRequests) handleRefreshToken(req *events.APIGatewayProxyRequest) Response {
	token := ""
	if req.QueryStringParameters != nil {
		token = req.QueryStringParameters["token"]
	}

	if token == "" {
		return p.jsonResponse(400, map[string]interface{}{
			"data":  nil,
			"error": "token query param is required",
		})
	}

	data, err := p.twitch.RefreshToken(token)
	if err != nil {
		return p.jsonResponse(200, map[string]interface{}{
			"data":  nil,
			"error": err.Error(),
		})
	}

	return p.jsonResponse(200, map[string]interface{}{"data": data, "error": nil})
}

func (p *ProxyRequests) handleVersion() Response {
	out := map[string]string{
		"deployedBy": "unknown",
		"deployedAt": "unknown",
		"gitSHA":     "unknown",
	}

	if p.config != nil {
		out["deployedBy"] = p.config.DeployedBy
		out["deployedAt"] = p.config.DeployedAt
		out["gitSHA"] = p.config.GitSHA
	}

	return p.jsonResponse(200, out)
}

func (p *ProxyRequests) jsonResponse(statusCode int, body interface{}) Response {
	raw, err := json.Marshal(body)
	if err != nil {
		raw = []byte(`{"error":"internal server error"}`)
	}
	return Response{
		StatusCode: statusCode,
		Headers:    DefaultHeaders(),
		Body:       string(raw),
	}
}

func htmlResponse(statusCode int, body string) Response {
	headers := DefaultHeaders()
	headers["Content-Type"] = "text/html"
	return Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}

func redirectPage(title, targetPrefix string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0" />
    <title>%s</title>
    <meta http-equiv="Cache-Control" content="no-cache, no-store, must-revalidate" />
    <meta http-equiv="Pragma" content="no-cache" />
    <meta http-equiv="Expires" content="0" />
  </head>
  <body>
    <h1>Redirecting…</h1>
    <p>If nothing happens automatically, return to Foam.</p>
    <a id="open-foam" href="%s">Open Foam</a>
    <script data-cfasync="false">
      const search = window.location.search.replace(/^\?/, '');
      const hash = window.location.hash.replace(/^#/, '');
      const params = new URLSearchParams(search);
      const hashParams = new URLSearchParams(hash);

      for (const [key, value] of hashParams.entries()) {
        params.set(key, value);
      }

      const query = params.toString();
      const redirectUrl = query ? '%s?' + query : '%s';
      const openFoam = document.getElementById('open-foam');

      if (openFoam) {
        openFoam.setAttribute('href', redirectUrl);
      }

      window.location.replace(redirectUrl);
      setTimeout(() => {
        window.location.href = redirectUrl;
      }, 150);
    </script>
  </body>
</html>`, title, targetPrefix, targetPrefix, targetPrefix)
}

func safeLog(message string, fields map[string]string) {
	body, _ := json.Marshal(fields)
	fmt.Printf(`{"level":"info","msg":%q,"fields":%s}`+"\n", message, string(body))
}
