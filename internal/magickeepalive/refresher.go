package magickeepalive

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

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
	return &Refresher{
		store:  store,
		twitch: twitch,
	}
}

// rotate the stored token so it stays fresh
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

	if resp.AccessToken == "" {
		return fmt.Errorf("twitch refresh returned no access token")
	}

	next := config.MagicLink{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		TokenType:    resp.TokenType,
	}

	// keep prior values if/when Twitch omits them from the response
	if next.RefreshToken == "" {
		next.RefreshToken = current.RefreshToken
	}

	if next.TokenType == "" {
		next.TokenType = current.TokenType
	}

	blob, err := json.Marshal(next)

	if err != nil {
		return fmt.Errorf("Failed to marshal blog: %w", err)
	}

	if err := r.store.Put(ctx, string(blob)); err != nil {
		return fmt.Errorf("Failed to write blob %w", err)
	}
	return nil
}
