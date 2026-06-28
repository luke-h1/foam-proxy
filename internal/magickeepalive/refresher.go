// Package magickeepalive keeps the App Review magic-link token alive. It reads the
// current token blob from the canonical store, exchanges its refresh token for a
// fresh access token at Twitch, and writes the rotated blob back — the work the
// old refresh-magic-link GitHub workflow did, now runnable from a scheduled
// Lambda.
package magickeepalive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

// ErrTokenRotated marks a failure after Twitch rotated the refresh token but
// before the new blob persisted. The old token is dead, so retrying is futile.
var ErrTokenRotated = errors.New("token rotated but blob not persisted")

type blobStore interface {
	Get(ctx context.Context) (string, error)
	Put(ctx context.Context, value string) error
}

type twitchRefresher interface {
	RefreshToken(token string) (*services.TwitchRefreshTokenResponse, error)
}

type Refresher struct {
	store  blobStore
	twitch twitchRefresher
}

func New(store blobStore, twitch twitchRefresher) *Refresher {
	return &Refresher{store: store, twitch: twitch}
}

// Refresh rotates the stored token. Twitch rotates the refresh token on use, so
// the old one dies immediately; persisting the new blob is mandatory, and any
// failure leaves the existing blob untouched.
func (r *Refresher) Refresh(ctx context.Context) error {
	raw, err := r.store.Get(ctx)
	if err != nil {
		return fmt.Errorf("read blob: %w", err)
	}

	current := config.ParseMagicLink(raw)
	if current == nil || current.RefreshToken == "" {
		return fmt.Errorf("stored blob has no refresh token")
	}

	resp, err := r.twitch.RefreshToken(current.RefreshToken)
	if err != nil {
		return fmt.Errorf("twitch refresh: %w", err)
	}
	if resp == nil {
		return fmt.Errorf("twitch refresh returned no response")
	}
	if resp.AccessToken == "" {
		return fmt.Errorf("twitch refresh returned no access token")
	}
	// Empty means Twitch rotated the old token away; carrying it forward would
	// persist a dead token.
	if resp.RefreshToken == "" {
		return fmt.Errorf("twitch refresh returned no refresh token")
	}

	next := config.MagicLink{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		TokenType:    resp.TokenType,
	}
	// Token type is constant ("bearer"), not rotating; keep prior if omitted.
	if next.TokenType == "" {
		next.TokenType = current.TokenType
	}

	blob, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf("marshal blob: %w", err)
	}
	// Token already rotated at Twitch; a failed write leaves SSM holding a dead
	// token. Tag it so the caller doesn't retry.
	if err := r.store.Put(ctx, string(blob)); err != nil {
		return errors.Join(ErrTokenRotated, fmt.Errorf("write blob: %w", err))
	}
	return nil
}
