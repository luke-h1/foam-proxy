package proxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
	"github.com/getsentry/sentry-go"
)

type magicLinkGetter interface {
	Get(ctx context.Context) (string, error)
}

// how often /api/magic re-reads SSM within one warm Lambda
// instance.
// short TTL keeps reviewers on a fresh token without hammering SSM.

const magicCacheTTL = 60 * time.Second

type ProxyRequests struct {
	config     *config.Proxy
	twitch     *services.TwitchService
	magicStore magicLinkGetter
	magicMu    sync.Mutex
	magicCache *config.MagicLink
	magicExp   time.Time
}

func NewProxyRequests(cfg *config.Proxy, twitch *services.TwitchService) *ProxyRequests {
	return &ProxyRequests{config: cfg, twitch: twitch}
}

func (p *ProxyRequests) resolveMagicLink() *config.MagicLink {
	if p.magicStore == nil {
		return p.config.MagicLink
	}

	p.magicMu.Lock()

	defer p.magicMu.Unlock()

	if !p.magicExp.IsZero() && time.Now().Before(p.magicExp) {
		return p.magicCache
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	raw, err := p.magicStore.Get(ctx)

	if err != nil {
		safeLog("[AUTHDBG] magic link SSM read failed", map[string]string{
			"error": err.Error(),
		})
		return p.config.MagicLink
	}
	parsed := config.ParseMagicLink(raw)
	if parsed == nil {
		return p.config.MagicLink
	}
	p.magicCache = parsed
	p.magicExp = time.Now().Add(magicCacheTTL)
	return p.magicCache
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
	var magic *config.MagicLink
	expectedKey := ""
	if p.config != nil {
		magic = p.resolveMagicLink()
		expectedKey = p.config.MagicLinkAPIKey
	}

	key := ""
	format := ""
	if req.QueryStringParameters != nil {
		key = req.QueryStringParameters["key"]
		format = req.QueryStringParameters["format"]
	}

	if magic == nil || expectedKey == "" || key == "" {
		return p.jsonResponse(404, map[string]string{"error": "not found"})
	}

	providedKey := sha256.Sum256([]byte(key))
	expected := sha256.Sum256([]byte(expectedKey))
	if subtle.ConstantTimeCompare(providedKey[:], expected[:]) != 1 {
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

func magicTokenType(magic *config.MagicLink) string {
	if magic.TokenType == "" {
		return "bearer"
	}
	return magic.TokenType
}

func magicTokenBlob(magic *config.MagicLink) map[string]interface{} {
	blob := map[string]interface{}{
		"access_token": magic.AccessToken,
		"token_type":   magicTokenType(magic),
	}
	if magic.RefreshToken != "" {
		blob["refresh_token"] = magic.RefreshToken
	}
	if magic.ExpiresIn > 0 {
		blob["expires_in"] = magic.ExpiresIn
	}
	return blob
}

func magicTargetURL(magic *config.MagicLink, scheme string) string {
	form := url.Values{}
	form.Set("access_token", magic.AccessToken)
	if magic.RefreshToken != "" {
		form.Set("refresh_token", magic.RefreshToken)
	}
	form.Set("token_type", magicTokenType(magic))

	if magic.ExpiresIn > 0 {
		form.Set("expires_in", strconv.Itoa(magic.ExpiresIn))
	}

	return scheme + "://auth?" + form.Encode()
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

func noStore(resp Response) Response {
	resp.Headers["Cache-Control"] = "no-store"
	return resp
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

func redirectTargetPage(title, target string) string {
	hrefAttr := html.EscapeString(target)
	jsTarget, _ := json.Marshal(target)

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
    <h1>Signing in…</h1>
    <p>If nothing happens automatically, return to Foam.</p>
    <a id="open-foam" href="%s">Open Foam</a>
    <script data-cfasync="false">
      const redirectUrl = %s;
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
</html>`, title, hrefAttr, string(jsTarget))
}

func safeLog(message string, fields map[string]string) {
	body, _ := json.Marshal(fields)
	fmt.Printf(`{"level":"info","msg":%q,"fields":%s}`+"\n", message, string(body))
}
