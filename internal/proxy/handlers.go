package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type Handlers struct {
	config *config.Proxy
	twitch map[string]tokenService
}

type tokenService interface {
	DefaultToken(ctx context.Context) (*services.TwitchTokenResponse, error)
	RefreshToken(ctx context.Context, token string) (*services.TwitchRefreshTokenResponse, error)
}

func NewHandlers(cfg *config.Proxy, twitch map[string]tokenService) *Handlers {
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

func (handlers *Handlers) Token(ctx context.Context, app string) (string, error) {
	service, err := handlers.serviceForApp(app)

	if err != nil {
		return errorBody(err), err
	}

	data, err := service.DefaultToken(ctx)

	if err != nil {
		return errorBody(err), err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"data":  data,
		"error": nil,
	})

	return string(body), nil
}

func (handlers *Handlers) RefreshToken(ctx context.Context, app, token string) (string, error) {
	service, err := handlers.serviceForApp(app)

	if err != nil {
		return errorBody(err), err
	}

	if token == "" {
		err := fmt.Errorf("token query param is required")
		return errorBody(err), err
	}

	data, err := service.RefreshToken(ctx, token)

	if err != nil {
		return errorBody(err), err
	}

	body, _ := json.Marshal(map[string]interface{}{
		"data":  data,
		"error": nil,
	})

	return string(body), nil
}

func (handlers *Handlers) Version(ctx context.Context) string {
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

func (handlers *Handlers) RedirectURI(app, requestURL string) (string, error) {
	appConfig, err := handlers.configForApp(app)

	if err != nil {
		return "", err
	}

	return buildRedirectURI(appConfig.RedirectURI, requestURL)
}

func buildRedirectURI(baseURI, requestURL string) (string, error) {
	request, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("invalid request URL")
	}

	baseParts := strings.SplitN(baseURI, "?", 2)
	query := url.Values{}
	if len(baseParts) == 2 {
		query, err = url.ParseQuery(baseParts[1])
		if err != nil {
			return "", fmt.Errorf("invalid redirect URI for app")
		}
	}

	for key, values := range request.Query() {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	if encoded := query.Encode(); encoded != "" {
		return baseParts[0] + "?" + encoded, nil
	}
	return baseParts[0], nil
}

func (handlers *Handlers) configForApp(app string) (*config.AppConfig, error) {
	if app == "" {
		return nil, fmt.Errorf("app query param is required")
	}

	if handlers.config == nil || handlers.config.Apps == nil {
		return nil, fmt.Errorf("unknown app %q", app)
	}

	appConfig, ok := handlers.config.Apps[app]

	if !ok {
		return nil, fmt.Errorf("unknown app %q", app)
	}

	return &appConfig, nil
}

func (handlers *Handlers) serviceForApp(app string) (tokenService, error) {
	if app == "" {
		return nil, fmt.Errorf("app query param is required")
	}

	if handlers.twitch == nil {
		return nil, fmt.Errorf("unknown app %q", app)
	}

	service, ok := handlers.twitch[app]
	if !ok || service == nil {
		return nil, fmt.Errorf("unknown app %q", app)
	}

	return service, nil
}

func errorBody(err error) string {
	body, _ := json.Marshal(map[string]interface{}{
		"data":  nil,
		"error": err.Error(),
	})

	return string(body)
}
