package proxy

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/observability"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type Handler struct {
	handlers *Handlers
	runtime  observability.ProxyRuntime
}

func NewHandler() (*Handler, error) {
	cfg, warnings, err := config.LoadEnv()
	if err != nil {
		log.Printf("Config load failed: %v", err)
		return nil, err
	}
	for _, warning := range warnings {
		log.Printf(`{"level":"warn","msg":"config warning","code":%q,"app":%q,"detail":%q}`, warning.Code, warning.App, warning.Message)
	}

	runtime := observability.NewRuntime("foam-proxy", cfg.Apps.Names())

	configuredApps := cfg.Apps.All()
	twitchServices := make(map[string]tokenService, len(configuredApps))

	for _, app := range configuredApps {
		twitchServices[app.Name] = services.NewTwitchService(
			app.Name,
			app.Config.TwitchClientID,
			app.Config.TwitchClientSecret,
			cfg.TwitchTimeout,
			runtime,
		)
	}

	handlers := NewHandlers(cfg, twitchServices, runtime)
	return &Handler{handlers: handlers, runtime: runtime}, nil
}

func (handler *Handler) HandleRequest(ctx context.Context, input *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	defer sentry.Flush(2 * time.Second)

	start := time.Now()

	if input == nil {
		return apiResponse(500, DefaultHeaders(), map[string]string{"error": "invalid request"}), nil
	}

	requestID := input.RequestContext.RequestID
	requestURL := buildRequestURL(input)
	method := input.RequestContext.HTTPMethod
	if method == "" {
		method = "GET"
	}
	tx := sentry.StartTransaction(ctx, method+" "+input.Path,
		sentry.WithOpName("http.server"),
		sentry.WithTransactionSource(sentry.SourceRoute),
	)
	defer tx.Finish()

	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("request_id", requestID)
		scope.SetTag("path", input.Path)
	})
	log.Printf(`{"level":"info","msg":"request","request_id":%q,"path":%q,"url":%q}`, requestID, input.Path, requestURL)

	status, headers, body := handler.handlers.Route(ctx, input.Path, requestURL, input.QueryStringParameters)

	app := ""
	if input.QueryStringParameters != nil {
		app = input.QueryStringParameters["app"]
	}

	handler.runtime.RecordRequest(ctx, input.Path, app, status, time.Since(start))
	pushCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := handler.runtime.Push(pushCtx); err != nil {
		log.Printf("metrics push failed: %v", err)
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    headers,
		Body:       body,
	}, nil
}

func apiResponse(status int, headers map[string]string, body interface{}) *events.APIGatewayProxyResponse {
	raw, err := json.Marshal(body)
	if err != nil {
		raw = []byte(`{"error":"internal server error"}`)
	}
	return &events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    headers,
		Body:       string(raw),
	}
}

func buildRequestURL(req *events.APIGatewayProxyRequest) string {
	host := "unknown"
	if req.Headers != nil {
		if v := req.Headers["Host"]; v != "" {
			host = v
		} else if v = req.Headers["host"]; v != "" {
			host = v
		}
	}
	path := req.Path
	if path == "" {
		path = "/"
	}
	q := ""
	if len(req.QueryStringParameters) > 0 {
		params := make(url.Values)
		for k, v := range req.QueryStringParameters {
			params.Set(k, v)
		}
		q = "?" + params.Encode()
	}
	return "https://" + host + path + q
}

func InitSentry() {
	dsn := os.Getenv("PROXY_DSN")
	if dsn == "" {
		return
	}
	if err := sentry.Init(config.SentryOptions(dsn)); err != nil {
		log.Printf("sentry init: %v", err)
	}
}
