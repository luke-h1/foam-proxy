package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/magiclink"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

type Handler struct {
	proxyRequests *Requests
}

const sentryFlushTimeout = 250 * time.Millisecond

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
		case "key":
			out[key] = "[redacted]"
		case "access_token", "code", "refresh_token", "state", "token":
			out[key] = redactValue(value)
		default:
			out[key] = value
		}
	}
	return out
}

func NewHandler() (*Handler, error) {
	cfg, err := config.LoadEnv()
	if err != nil {
		return nil, err
	}

	twitchService := services.NewTwitchService(cfg.TwitchClientID, cfg.TwitchClientSecret, cfg.TwitchTimeout)
	proxyRequests := NewRequests(cfg, twitchService)

	if cfg.MagicLinkSSMParam != "" {
		store, storeErr := magiclink.NewStore(context.Background(), cfg.MagicLinkSSMParam)
		if storeErr != nil {
			if cfg.MagicLink == nil {
				return nil, fmt.Errorf("magic link SSM store init failed: %w", storeErr)
			}
			slog.Error("magic link SSM store init failed, using env blob fallback", "err", storeErr)
		} else {
			proxyRequests.setMagicStore(store)
		}
	}

	return &Handler{proxyRequests: proxyRequests}, nil
}

func (handler *Handler) HandleRequest(ctx context.Context, input *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	flushFor := 20 * time.Millisecond
	defer func() { sentry.Flush(flushFor) }()

	if input == nil {
		flushFor = sentryFlushTimeout
		return apiResponse(http.StatusInternalServerError, DefaultHeaders(), map[string]string{"error": "invalid request"}), nil
	}

	hub := sentry.CurrentHub().Clone()
	ctx = sentry.SetHubOnContext(ctx, hub)

	requestID := input.RequestContext.RequestID
	requestURL := buildRequestURL(input)
	method := input.RequestContext.HTTPMethod
	if method == "" {
		method = http.MethodGet
	}

	hub.Scope().SetTag("request_id", requestID)
	hub.Scope().SetTag("path", input.Path)
	hub.Scope().SetTag("http.method", method)
	hub.Scope().SetContext("request", sentry.Context{
		"id":     requestID,
		"url":    requestURL,
		"method": method,
		"path":   input.Path,
		"query":  sanitizeQuery(input.QueryStringParameters),
	})

	isHealth := input.Path == "/api/healthcheck"
	var tx *sentry.Span
	if !isHealth {
		tx = sentry.StartTransaction(ctx, method+" "+input.Path,
			sentry.WithOpName("http.server"),
			sentry.WithTransactionSource(sentry.SourceRoute),
		)
		defer tx.Finish()
		ctx = sentry.SetHubOnContext(tx.Context(), hub)
	}

	start := time.Now()
	slog.Info("request", "request_id", requestID, "path", input.Path, "method", method)
	response := handler.proxyRequests.Handle(ctx, input)
	elapsed := time.Since(start)
	slog.Info("response",
		"request_id", requestID,
		"path", input.Path,
		"status", response.StatusCode,
		"duration_ms", elapsed.Milliseconds(),
	)

	if response.StatusCode >= http.StatusInternalServerError {
		flushFor = sentryFlushTimeout
	}

	if tx != nil {
		tx.Status = spanStatusForHTTP(response.StatusCode)
		meter := sentry.NewMeter(ctx)
		meter.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", input.Path),
		)
		statusAttr := sentry.WithAttributes(attribute.Int("http.status_code", response.StatusCode))
		meter.Count("proxy.request", 1)
		meter.Count("proxy.response", 1, statusAttr)
		meter.Distribution("proxy.request.duration_ms", float64(elapsed.Milliseconds()), sentry.WithUnit(sentry.UnitMillisecond), statusAttr)
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: response.StatusCode,
		Headers:    response.Headers,
		Body:       response.Body,
	}, nil
}

func spanStatusForHTTP(status int) sentry.SpanStatus {
	switch {
	case status >= http.StatusOK && status < http.StatusBadRequest:
		return sentry.SpanStatusOK
	case status == http.StatusUnauthorized:
		return sentry.SpanStatusUnauthenticated
	case status == http.StatusForbidden:
		return sentry.SpanStatusPermissionDenied
	case status == http.StatusNotFound:
		return sentry.SpanStatusNotFound
	case status == http.StatusTooManyRequests:
		return sentry.SpanStatusResourceExhausted
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return sentry.SpanStatusInvalidArgument
	default:
		return sentry.SpanStatusInternalError
	}
}

func apiResponse(status int, headers map[string]string, body any) *events.APIGatewayProxyResponse {
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
		for k, v := range sanitizeQuery(req.QueryStringParameters) {
			params.Set(k, v)
		}
		q = "?" + params.Encode()
	}
	return "https://" + host + path + q
}

func InitSentry() {
	config.InitSentry("PROXY_DSN")
}
