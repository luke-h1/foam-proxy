package proxy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
)

type fakeMagicSource struct {
	calls int
	raw   string
	err   error
}

func (f *fakeMagicSource) Get(ctx context.Context) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.raw, nil
}

const testBlob = `{"access_token":"ABC","refresh_token":"REF"}`

func TestMagicLinkCacheFallsBackToEnvWithoutSource(t *testing.T) {
	envLink := &config.MagicLink{AccessToken: "ENV", RefreshToken: "REF"}
	cache := newMagicLinkCache(nil, envLink)

	if got := cache.Resolve(); got != envLink {
		t.Fatalf("Resolve() = %v, want env link", got)
	}
}

func TestMagicLinkCacheCachesWithinTTL(t *testing.T) {
	source := &fakeMagicSource{raw: testBlob}
	cache := newMagicLinkCache(source, nil)

	first := cache.Resolve()
	second := cache.Resolve()

	if first == nil || first.AccessToken != "ABC" {
		t.Fatalf("Resolve() = %v, want parsed blob", first)
	}
	if second != first {
		t.Fatalf("second Resolve() = %v, want cached %v", second, first)
	}
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1 (second read served from cache)", source.calls)
	}
}

func TestMagicLinkCacheNegativeCachesSourceErrors(t *testing.T) {
	envLink := &config.MagicLink{AccessToken: "ENV", RefreshToken: "REF"}
	source := &fakeMagicSource{err: errors.New("ssm down")}
	cache := newMagicLinkCache(source, envLink)

	if got := cache.Resolve(); got != envLink {
		t.Fatalf("Resolve() = %v, want env fallback", got)
	}
	if got := cache.Resolve(); got != envLink {
		t.Fatalf("second Resolve() = %v, want env fallback", got)
	}
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1 (error should be negative-cached)", source.calls)
	}
}

func TestMagicLinkCacheNegativeCachesParseFailures(t *testing.T) {
	source := &fakeMagicSource{raw: "not json"}
	cache := newMagicLinkCache(source, nil)

	if got := cache.Resolve(); got != nil {
		t.Fatalf("Resolve() = %v, want nil", got)
	}
	if got := cache.Resolve(); got != nil {
		t.Fatalf("second Resolve() = %v, want nil", got)
	}
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1 (parse failure should be negative-cached)", source.calls)
	}
}

func newMagicProxy(source *fakeMagicSource) *ProxyRequests {
	p := NewProxyRequests(&config.Proxy{MagicLinkAPIKey: "secret"}, nil)
	if source != nil {
		p.setMagicStore(source)
	}
	return p
}

func magicRequest(params map[string]string) *events.APIGatewayProxyRequest {
	return &events.APIGatewayProxyRequest{Path: "/api/magic", QueryStringParameters: params}
}

func TestHandleMagicRejectsMissingKeyWithoutCacheRead(t *testing.T) {
	source := &fakeMagicSource{raw: testBlob}
	p := newMagicProxy(source)

	resp := p.Handle(magicRequest(nil))
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0 (no cache read before auth)", source.calls)
	}
}

func TestHandleMagicRejectsWrongKeyWithoutCacheRead(t *testing.T) {
	source := &fakeMagicSource{raw: testBlob}
	p := newMagicProxy(source)

	resp := p.Handle(magicRequest(map[string]string{"key": "wrong"}))
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0 (no cache read before auth)", source.calls)
	}
}

func TestHandleMagicServesJSONForValidKey(t *testing.T) {
	p := newMagicProxy(&fakeMagicSource{raw: testBlob})

	resp := p.Handle(magicRequest(map[string]string{"key": "secret", "format": "json"}))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, `"access_token":"ABC"`) {
		t.Fatalf("body = %q, want access token blob", resp.Body)
	}
	if resp.Headers["Cache-Control"] != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", resp.Headers["Cache-Control"])
	}
}

func TestHandleMagicServesRedirectForValidKey(t *testing.T) {
	p := newMagicProxy(&fakeMagicSource{raw: testBlob})

	resp := p.Handle(magicRequest(map[string]string{"key": "secret"}))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if resp.Headers["Content-Type"] != "text/html" {
		t.Fatalf("Content-Type = %q, want text/html", resp.Headers["Content-Type"])
	}
	if !strings.Contains(resp.Body, "foam://auth") {
		t.Fatalf("body = %q, want foam://auth redirect", resp.Body)
	}
}

func TestHandleMagicHiddenWhenNoBlobAvailable(t *testing.T) {
	p := newMagicProxy(&fakeMagicSource{err: errors.New("ssm down")})

	resp := p.Handle(magicRequest(map[string]string{"key": "secret"}))
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404 when no blob resolves", resp.StatusCode)
	}
}
