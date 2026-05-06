package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/observability"
	"github.com/foam/proxy/internal/proxy/services"
)

var errNotFound = fmt.Errorf("not found")

type Handlers struct {
	config  *config.Proxy
	twitch  map[string]tokenService
	runtime observability.ProxyRuntime
}

type tokenService interface {
	DefaultToken(ctx context.Context) (*services.TwitchTokenResponse, error)
	RefreshToken(ctx context.Context, token string) (*services.TwitchRefreshTokenResponse, error)
}

func NewHandlers(cfg *config.Proxy, twitch map[string]tokenService, runtime observability.ProxyRuntime) *Handlers {
	return &Handlers{config: cfg, twitch: twitch, runtime: runtime}
}

func (handlers *Handlers) Health() string {
	handlers.runtime.RecordHealthCheck(context.Background())

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
		return clientErrorBody(err), err
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
		return clientErrorBody(err), err
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
	if handlers.config == nil {
		return nil, fmt.Errorf("config unavailable")
	}

	configuredApp, err := handlers.config.Apps.Get(app)
	if err != nil {
		return nil, err
	}

	appConfig := configuredApp.Config
	return &appConfig, nil
}

func (handlers *Handlers) serviceForApp(app string) (tokenService, error) {
	configuredApp, err := handlers.config.Apps.Get(app)
	if err != nil {
		return nil, err
	}

	if handlers.twitch == nil {
		return nil, fmt.Errorf("token service unavailable")
	}

	service, ok := handlers.twitch[configuredApp.Name]
	if !ok || service == nil {
		return nil, fmt.Errorf("token service unavailable")
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

func clientErrorBody(err error) string {
	if lookupErr, ok := err.(*config.AppLookupError); ok {
		return errorBody(fmt.Errorf("%s", clientErrorMessage(lookupErr)))
	}

	return errorBody(err)
}

func clientErrorMessage(err *config.AppLookupError) string {
	switch err.Code {
	case config.AppLookupMissingApp:
		return "app query param is required"
	case config.AppLookupInvalidApp:
		return "invalid app"
	default:
		return "invalid request"
	}
}
