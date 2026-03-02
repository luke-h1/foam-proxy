package config

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultTwitchTimeout = 20
)

type Proxy struct {
	TwitchClientID     string
	TwitchClientSecret string
	TwitchTimeout      time.Duration
	DeployedBy         string
	DeployedAt         string
	GitSHA             string
}

func LoadEnv() (*Proxy, error) {
	clientId := os.Getenv("TWITCH_CLIENT_ID")
	clientSecret := os.Getenv("TWITCH_CLIENT_SECRET")

	if clientId == "" || clientSecret == "" {
		return nil, fmt.Errorf("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET are required")
	}

	return &Proxy{
		TwitchClientID:     clientId,
		TwitchClientSecret: clientSecret,
		TwitchTimeout:      defaultTwitchTimeout,
		DeployedBy:         os.Getenv("DEPLOYED_BY"),
		DeployedAt:         os.Getenv("DEPLOYED_AT"),
		GitSHA:             os.Getenv("GIT_SHA"),
	}, nil
}
