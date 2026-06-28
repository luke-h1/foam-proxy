package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/magiclink"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type Handler struct {
	proxyRequests *ProxyRequests
}

func redactValue(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return value
	}
	return value[:8] + "…"
}

func sanitizeQuery(query map[string]string) map[string]string {
	if len(query) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(query))
	for key, value := range query {
		switch key {
		case "access_token", "code", "refresh_token", "state":
			out[key] = redactValue(value)
		default:
			out[key] = value
		}
	}
	return out
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func NewHandler() (*Handler, error) {
	cfg, err := config.LoadEnv()
	if err != nil {
		return nil, err
	}

	twitchService := services.NewTwitchService(cfg.TwitchClientID, cfg.TwitchClientSecret, cfg.TwitchTimeout)
	proxyRequests := NewProxyRequests(cfg, twitchService)

	// Read the canonical magic-link blob from SSM at request time. Without a param
	// (local/dev) the handler falls back to the MAGIC_LINK_BLOB env var.
	if cfg.MagicLinkSSMParam != "" {
		store, storeErr := magiclink.NewStore(context.Background(), cfg.MagicLinkSSMParam)
		if storeErr != nil {
			// No env fallback (prod): a nil store silently 404s /api/magic, so
			// fail fast rather than look like "feature disabled".
			if cfg.MagicLink == nil {
				return nil, fmt.Errorf("magic link SSM store init failed: %w", storeErr)
			}
			log.Printf("magic link SSM store init failed, using env blob fallback: %v", storeErr)
		} else {
			proxyRequests.magicStore = store
		}
	}

	return &Handler{proxyRequests: proxyRequests}, nil
}

func (handler *Handler) HandleRequest(ctx context.Context, input *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	defer sentry.Flush(2 * time.Second)

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
	log.Printf(`{"level":"info","msg":"request","request_id":%q,"path":%q,"url":%q,"method":%q,"query":%s,"query_keys":%q}`, requestID, input.Path, requestURL, method, mustJSON(sanitizeQuery(input.QueryStringParameters)), sortedKeys(input.QueryStringParameters))
	response := handler.proxyRequests.Handle(input)
	log.Printf(`{"level":"info","msg":"response","request_id":%q,"path":%q,"status":%d,"content_type":%q,"location":%q,"body_preview":%q}`, requestID, input.Path, response.StatusCode, response.Headers["Content-Type"], response.Headers["Location"], bodyPreview(response.Body))
	return &events.APIGatewayProxyResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.Body,
	}, nil
}

func bodyPreview(body string) string {
	if len(body) <= 240 {
		return body
	}
	return body[:240] + "…"
}

func mustJSON(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(raw)
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
