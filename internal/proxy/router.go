package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type ProxyRequests struct {
	config     *config.Proxy
	twitch     *services.TwitchService
	magicCache *magicLinkCache
}

func NewProxyRequests(cfg *config.Proxy, twitch *services.TwitchService) *ProxyRequests {
	var envLink *config.MagicLink
	if cfg != nil {
		envLink = cfg.MagicLink
	}
	return &ProxyRequests{config: cfg, twitch: twitch, magicCache: newMagicLinkCache(nil, envLink)}
}

func (p *ProxyRequests) setMagicStore(source magicLinkSource) {
	p.magicCache.source = source
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
	case "/api/magic":
		return p.handleMagic(req)
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

func (p *ProxyRequests) handleMagic(req *events.APIGatewayProxyRequest) Response {
	expectedKey := ""
	if p.config != nil {
		expectedKey = p.config.MagicLinkAPIKey
	}

	key := ""
	format := ""
	if req.QueryStringParameters != nil {
		key = req.QueryStringParameters["key"]
		format = req.QueryStringParameters["format"]
	}

	if expectedKey == "" || key == "" {
		return p.jsonResponse(404, map[string]string{"error": "not found"})
	}

	providedKey := sha256.Sum256([]byte(key))
	expected := sha256.Sum256([]byte(expectedKey))
	if subtle.ConstantTimeCompare(providedKey[:], expected[:]) != 1 {
		return p.jsonResponse(404, map[string]string{"error": "not found"})
	}

	// only authenticated requests pay for the SSM-backed cache read
	magic := p.magicCache.Resolve()
	if magic == nil {
		return p.jsonResponse(404, map[string]string{"error": "not found"})
	}

	if format == "json" {
		safeLog("[AUTHDBG] magic link served", map[string]string{
			"target": "json",
		})
		return noStore(p.jsonResponse(200, magicTokenBlob(magic)))
	}

	scheme := resolveScheme(req)
	safeLog("[AUTHDBG] magic link served", map[string]string{
		"target": scheme + "://auth",
	})

	// return deep-link for app-store reviewer
	return noStore(htmlResponse(200, redirectTargetPage("Foam - Signing in", magicTargetURL(magic, scheme))))
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

func safeLog(message string, fields map[string]string) {
	body, _ := json.Marshal(fields)
	fmt.Printf(`{"level":"info","msg":%q,"fields":%s}`+"\n", message, string(body))
}
