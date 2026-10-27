// Package magickeepalive keeps the App Review magic-link token alive. It reads the
// current token blob from the store, exchanges its refresh token for a
// fresh access token at Twitch, and writes the rotated blob back
package magickeepalive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

var ErrTokenRotated = errors.New("magickeepalive: token rotated but blob not persisted")

type blobStore interface {
	Get(ctx context.Context) (string, error)
	Put(ctx context.Context, value string) error
}

type twitchRefresher interface {
	RefreshToken(ctx context.Context, token string) (*services.TwitchRefreshTokenResponse, error)
}

type Refresher struct {
	store  blobStore
	twitch twitchRefresher
}

func New(store blobStore, twitch twitchRefresher) *Refresher {
	return &Refresher{store: store, twitch: twitch}
}

func (r *Refresher) Refresh(ctx context.Context) error {
	raw, err := r.store.Get(ctx)
	if err != nil {
		return fmt.Errorf("read blob: %w", err)
	}

	current := config.ParseMagicLink(raw)

	if current == nil || current.RefreshToken == "" {
		return errors.New("stored blob has no refresh token")
	}

	resp, err := r.twitch.RefreshToken(ctx, current.RefreshToken)
	if err != nil {
		return fmt.Errorf("twitch refresh: %w", err)
	}

	if resp == nil {
		return errors.New("twitch refresh returned no response")
	}

	if resp.AccessToken == "" {
		return errors.New("twitch refresh returned no access token")
	}
	if resp.RefreshToken == "" {
		return errors.New("twitch refresh returned no refresh token")
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

	blob, err := json.Marshal(next) //nolint:gosec // G117: persisting the rotated token is what this write is for
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
