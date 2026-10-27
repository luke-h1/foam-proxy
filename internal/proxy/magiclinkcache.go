package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/foam/proxy/internal/config"
	"golang.org/x/sync/singleflight"
)

type magicLinkSource interface {
	Get(ctx context.Context) (string, error)
}

const (
	magicCacheTTL      = 60 * time.Second
	magicCacheErrorTTL = 5 * time.Second
	magicFetchTimeout  = 1500 * time.Millisecond
)

type magicLinkCache struct {
	source  magicLinkSource
	envLink *config.MagicLink

	mu     sync.Mutex
	cached *config.MagicLink
	exp    time.Time
	flight singleflight.Group
}

func newMagicLinkCache(source magicLinkSource, envLink *config.MagicLink) *magicLinkCache {
	return &magicLinkCache{source: source, envLink: envLink}
}

// Resolve returns the current token blob, reading SSM (cached) when a store is
// configured and otherwise falling back to the env-var blob. A transient SSM
// error also falls back to the env blob rather than breaking the review login.
func (c *magicLinkCache) Resolve(ctx context.Context) *config.MagicLink {
	if c.source == nil {
		return c.envLink
	}

	if link, ok := c.fresh(); ok {
		return link
	}

	v, _, _ := c.flight.Do("magic", func() (any, error) {
		if link, ok := c.fresh(); ok {
			return link, nil
		}
		return c.load(ctx), nil
	})

	link, ok := v.(*config.MagicLink)
	if !ok {
		return c.envLink
	}
	return link
}

func (c *magicLinkCache) fresh() (*config.MagicLink, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.exp.IsZero() || !time.Now().Before(c.exp) {
		return nil, false
	}
	if c.cached != nil {
		return c.cached, true
	}
	return c.envLink, true
}

func (c *magicLinkCache) load(ctx context.Context) *config.MagicLink {
	fetchCtx, cancel := context.WithTimeout(ctx, magicFetchTimeout)
	defer cancel()

	raw, err := c.source.Get(fetchCtx)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		// cancelled request should not mutate the cache
		if ctx.Err() != nil {
			if c.cached != nil {
				return c.cached
			}
			return c.envLink
		}
		safeLog("[AUTHDBG] magic link SSM read failed", map[string]string{"error": err.Error()})
		c.exp = time.Now().Add(magicCacheErrorTTL)
		if c.cached != nil {
			return c.cached
		}
		return c.envLink
	}

	parsed := config.ParseMagicLink(raw)
	if parsed == nil {
		safeLog("[AUTHDBG] magic link SSM parse failed", map[string]string{})
		c.exp = time.Now().Add(magicCacheErrorTTL)
		if c.cached != nil {
			return c.cached
		}
		return c.envLink
	}

	c.cached = parsed
	c.exp = time.Now().Add(magicCacheTTL)
	return c.cached
}
