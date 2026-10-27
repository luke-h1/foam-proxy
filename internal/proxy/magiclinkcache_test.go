package proxy

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
)

type fakeMagicSource struct {
	calls int
	raw   string
	err   error
}

func (f *fakeMagicSource) Get(_ context.Context) (string, error) {
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

	if got := cache.Resolve(context.Background()); got != envLink {
		t.Fatalf("Resolve() = %v, want env link", got)
	}
}

func TestMagicLinkCacheCachesWithinTTL(t *testing.T) {
	source := &fakeMagicSource{raw: testBlob}
	cache := newMagicLinkCache(source, nil)

	first := cache.Resolve(context.Background())
	second := cache.Resolve(context.Background())

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

	if got := cache.Resolve(context.Background()); got != envLink {
		t.Fatalf("Resolve() = %v, want env fallback", got)
	}
	if got := cache.Resolve(context.Background()); got != envLink {
		t.Fatalf("second Resolve() = %v, want env fallback", got)
	}
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1 (error should be negative-cached)", source.calls)
	}
}

func TestMagicLinkCacheNegativeCachesParseFailures(t *testing.T) {
	source := &fakeMagicSource{raw: "not json"}
	cache := newMagicLinkCache(source, nil)

	if got := cache.Resolve(context.Background()); got != nil {
		t.Fatalf("Resolve() = %v, want nil", got)
	}
	if got := cache.Resolve(context.Background()); got != nil {
		t.Fatalf("second Resolve() = %v, want nil", got)
	}
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1 (parse failure should be negative-cached)", source.calls)
	}
}

func newMagicProxy(source *fakeMagicSource) *Requests {
	p := NewRequests(&config.Proxy{MagicLinkAPIKey: "secret"}, nil)
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

	resp := p.Handle(context.Background(), magicRequest(nil))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0 (no cache read before auth)", source.calls)
	}
}

func TestHandleMagicRejectsWrongKeyWithoutCacheRead(t *testing.T) {
	source := &fakeMagicSource{raw: testBlob}
	p := newMagicProxy(source)

	resp := p.Handle(context.Background(), magicRequest(map[string]string{"key": "wrong"}))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	if source.calls != 0 {
		t.Fatalf("source calls = %d, want 0 (no cache read before auth)", source.calls)
	}
}

func TestHandleMagicServesJSONForValidKey(t *testing.T) {
	p := newMagicProxy(&fakeMagicSource{raw: testBlob})

	resp := p.Handle(context.Background(), magicRequest(map[string]string{"key": "secret", "format": "json"}))
	if resp.StatusCode != http.StatusOK {
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

	resp := p.Handle(context.Background(), magicRequest(map[string]string{"key": "secret"}))
	if resp.StatusCode != http.StatusOK {
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

	resp := p.Handle(context.Background(), magicRequest(map[string]string{"key": "secret"}))
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 when no blob resolves", resp.StatusCode)
	}
}

type blockingMagicSource struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (b *blockingMagicSource) Get(ctx context.Context) (string, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()

	select {
	case <-b.started:
	default:
		close(b.started)
	}

	select {
	case <-b.release:
		return testBlob, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestMagicLinkCacheCoalescesConcurrentReads(t *testing.T) {
	source := &blockingMagicSource{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	cache := newMagicLinkCache(source, nil)

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n)
	for range n {
		go func() {
			defer wg.Done()
			got := cache.Resolve(context.Background())
			if got == nil || got.AccessToken != "ABC" {
				errCh <- errors.New("missing cached blob")
			}
		}()
	}

	<-source.started
	close(source.release)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	if source.calls != 1 {
		t.Fatalf("source calls = %d, want 1", source.calls)
	}
}
