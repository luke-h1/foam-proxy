package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type twitchClient interface {
	DefaultToken(ctx context.Context) (*services.TwitchTokenResponse, error)
	RefreshToken(ctx context.Context, token string) (*services.TwitchRefreshTokenResponse, error)
}

type Requests struct {
	config       *config.Proxy
	twitch       twitchClient
	magicCache   *magicLinkCache
	magicKeyHash [32]byte
	hasMagicKey  bool
}

func NewRequests(cfg *config.Proxy, twitch twitchClient) *Requests {
	var envLink *config.MagicLink
	p := &Requests{config: cfg, twitch: twitch}
	if cfg != nil {
		envLink = cfg.MagicLink
		if cfg.MagicLinkAPIKey != "" {
			p.magicKeyHash = sha256.Sum256([]byte(cfg.MagicLinkAPIKey))
			p.hasMagicKey = true
		}
	}
	p.magicCache = newMagicLinkCache(nil, envLink)
	return p
}

func (p *Requests) setMagicStore(source magicLinkSource) {
	p.magicCache.source = source
}

func (p *Requests) Handle(ctx context.Context, req *events.APIGatewayProxyRequest) Response {
	if req == nil {
		return p.jsonResponse(http.StatusInternalServerError, map[string]string{"error": "invalid request"})
	}

	switch req.Path {
	case "/api/pending":
		return p.handlePending()
	case "/api/proxy":
		return p.handleProxy()
	case "/api/token":
		return p.handleToken(ctx)
	case "/api/refresh-token":
		return p.handleRefreshToken(ctx, req)
	case "/api/magic":
		return p.handleMagic(ctx, req)
	case "/api/healthcheck":
		return p.handleHealth(ctx)
	case "/api/version":
		return p.handleVersion()
	default:
		return p.jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (p *Requests) handleHealth(ctx context.Context) Response {
	sentry.NewMeter(ctx).Count("health.check", 1)

	return p.jsonResponse(http.StatusOK, map[string]string{
		"status": "OK",
	})
}

func (p *Requests) handlePending() Response {
	safeLog("[AUTHDBG] pending page served", map[string]string{
		"target": "foam://",
	})
	return htmlResponse(http.StatusOK, redirectPage("Foam - Pending", "foam://"))
}

func (p *Requests) handleProxy() Response {
	safeLog("[AUTHDBG] proxy page served", map[string]string{
		"target": "foam://",
	})
	return htmlResponse(http.StatusOK, redirectPage("Foam - Redirecting", "foam://"))
}

// resolveScheme picks the magic-link redirect scheme from the request's ?scheme
// query param, validated against the variant allowlist; anything else falls back
// to the production scheme.
func resolveScheme(req *events.APIGatewayProxyRequest) string {
	requested := ""
	if req != nil && req.QueryStringParameters != nil {
		requested = req.QueryStringParameters["scheme"]
	}
	return config.ResolveAppScheme(requested)
}

func (p *Requests) handleToken(ctx context.Context) Response {
	data, err := p.twitch.DefaultToken(ctx)
	if err != nil {
		return p.jsonResponse(http.StatusBadGateway, map[string]any{
			"data":  nil,
			"error": err.Error(),
		})
	}

	return p.jsonResponse(http.StatusOK, map[string]any{"data": data, "error": nil})
}

func (p *Requests) handleRefreshToken(ctx context.Context, req *events.APIGatewayProxyRequest) Response {
	token := ""
	if req.QueryStringParameters != nil {
		token = req.QueryStringParameters["token"]
	}

	if token == "" {
		return p.jsonResponse(http.StatusBadRequest, map[string]any{
			"data":  nil,
			"error": "token query param is required",
		})
	}

	data, err := p.twitch.RefreshToken(ctx, token)
	if err != nil {
		return p.jsonResponse(http.StatusBadGateway, map[string]any{
			"data":  nil,
			"error": err.Error(),
		})
	}

	return p.jsonResponse(http.StatusOK, map[string]any{"data": data, "error": nil})
}

func (p *Requests) handleMagic(ctx context.Context, req *events.APIGatewayProxyRequest) Response {
	key := ""
	format := ""
	if req.QueryStringParameters != nil {
		key = req.QueryStringParameters["key"]
		format = req.QueryStringParameters["format"]
	}

	if !p.hasMagicKey || key == "" {
		return p.jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	providedKey := sha256.Sum256([]byte(key))
	if subtle.ConstantTimeCompare(providedKey[:], p.magicKeyHash[:]) != 1 {
		return p.jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	magic := p.magicCache.Resolve(ctx)
	if magic == nil {
		return p.jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	if format == "json" {
		safeLog("[AUTHDBG] magic link served", map[string]string{
			"target": "json",
		})
		return noStore(p.jsonResponse(http.StatusOK, magicTokenBlob(magic)))
	}

	scheme := resolveScheme(req)
	safeLog("[AUTHDBG] magic link served", map[string]string{
		"target": scheme + "://auth",
	})

	return noStore(htmlResponse(http.StatusOK, redirectTargetPage("Foam - Signing in", magicTargetURL(magic, scheme))))
}

func (p *Requests) handleVersion() Response {
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

	return p.jsonResponse(http.StatusOK, out)
}

func safeLog(message string, fields map[string]string) {
	args := make([]any, 0, len(fields)*2)
	for key, value := range fields {
		args = append(args, key, value)
	}
	slog.Info(message, args...)
}
