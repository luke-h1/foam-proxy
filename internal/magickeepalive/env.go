package magickeepalive

import (
	"context"
	"fmt"
	"time"

	"github.com/foam/proxy/internal/magiclink"
	"github.com/foam/proxy/internal/proxy/services"
)

const twitchTimeout = 20 * time.Second

func NewFromEnv(ctx context.Context, getenv func(string) string) (*Refresher, error) {
	param := getenv("MAGIC_LINK_SSM_PARAM")
	if param == "" {
		return nil, fmt.Errorf("MAGIC_LINK_SSM_PARAM is required")
	}

	clientID := getenv("TWITCH_CLIENT_ID")
	clientSecret := getenv("TWITCH_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}

	store, err := magiclink.NewStore(ctx, param)
	if err != nil {
		return nil, fmt.Errorf("init ssm store: %w", err)
	}

	twitch := services.NewTwitchService(clientID, clientSecret, twitchTimeout)
	return New(store, twitch), nil
}
