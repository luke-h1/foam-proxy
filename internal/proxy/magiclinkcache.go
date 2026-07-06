package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/foam/proxy/internal/config"
)

// magicLinkSource reads the raw token blob JSON from the canonical store (SSM).
type magicLinkSource interface {
	Get(ctx context.Context) (string, error)
}

const magicCacheTTL = 60 * time.Second

type magicLinkCache struct {
	source  magicLinkSource
	envLink *config.MagicLink

	mu     sync.Mutex
	cached *config.MagicLink
	exp    time.Time
}

func newMagicLinkCache(source magicLinkSource, envLink *config.MagicLink) *magicLinkCache {
	return &magicLinkCache{source: source, envLink: envLink}
}

// Resolve returns the current token blob, reading SSM (cached) when a store is
// configured and otherwise falling back to the env-var blob. A transient SSM
// error also falls back to the env blob rather than breaking the review login.
func (c *magicLinkCache) Resolve() *config.MagicLink {
	if c.source == nil {
		return c.envLink
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.exp.IsZero() && time.Now().Before(c.exp) {
		return c.cached
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raw, err := c.source.Get(ctx)
	if err != nil {
		safeLog("[AUTHDBG] magic link SSM read failed", map[string]string{"error": err.Error()})
		if c.cached != nil {
			return c.cached
		}
		return c.envLink
	}

	parsed := config.ParseMagicLink(raw)
	if parsed == nil {
		safeLog("[AUTHDBG] magic link SSM parse failed", map[string]string{})
		if c.cached != nil {
			return c.cached
		}
		return c.envLink
	}

	c.cached = parsed
	c.exp = time.Now().Add(magicCacheTTL)
	return c.cached
}
