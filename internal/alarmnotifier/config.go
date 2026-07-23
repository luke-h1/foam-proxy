package alarmnotifier

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DiscordWebhookURL string
	TelegramBotToken  string
	TelegramChatID    string
	Environment       string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		DiscordWebhookURL: strings.TrimSpace(os.Getenv("DISCORD_WEBHOOK_URL")),
		TelegramBotToken:  strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		TelegramChatID:    strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")),
		Environment:       strings.TrimSpace(os.Getenv("SENTRY_ENVIRONMENT")),
	}

	if cfg.Environment == "" {
		cfg.Environment = strings.TrimSpace(os.Getenv("ENVIRONMENT"))
	}

	if (cfg.TelegramBotToken == "") != (cfg.TelegramChatID == "") {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID must both be set or both empty")
	}
	if !cfg.HasDiscord() && !cfg.HasTelegram() {
		return nil, fmt.Errorf("at least one of DISCORD_WEBHOOK_URL or TELEGRAM_BOT_TOKEN+TELEGRAM_CHAT_ID is required")
	}

	return cfg, nil
}

func (c *Config) HasDiscord() bool {
	return c.DiscordWebhookURL != ""
}

func (c *Config) HasTelegram() bool {
	return c.TelegramBotToken != "" && c.TelegramChatID != ""
}
