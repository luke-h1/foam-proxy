package alarmnotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	httpClient         = &http.Client{Timeout: 10 * time.Second}
	telegramAPIBaseURL = "https://api.telegram.org"
)

type Notifier struct {
	cfg    *Config
	client *http.Client
}

func NewNotifier(cfg *Config) *Notifier {
	return &Notifier{cfg: cfg, client: httpClient}
}

func (n *Notifier) Notify(ctx context.Context, alarm *AlarmNotification) error {
	var errs []string

	if n.cfg.HasDiscord() {
		if err := n.sendDiscord(ctx, alarm); err != nil {
			errs = append(errs, fmt.Sprintf("Discord: %v", err))
		}
	}

	if n.cfg.HasTelegram() {
		if err := n.sendTelegram(ctx, alarm); err != nil {
			errs = append(errs, fmt.Sprintf("telegram: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("Notify failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (n *Notifier) sendDiscord(ctx context.Context, alarm *AlarmNotification) error {
	payload := map[string]any{
		"content": fmt.Sprintf("CW alarm **%s** -> `%s`", alarm.AlarmName, alarm.NewStateValue),
		"embeds": []map[string]any{
			{
				"title":       fmt.Sprintf("CW: %s", alarm.NewStateValue),
				"description": FormatPlainText(alarm, n.cfg.Environment),
				"color":       discordColor(alarm.NewStateValue),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.DiscordWebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create discord request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := doJSON(n.client, req); err != nil {
		return fmt.Errorf("send discord webhook: %w", err)
	}
	return nil
}

func (n *Notifier) sendTelegram(ctx context.Context, alarm *AlarmNotification) error {
	payload := map[string]any{
		"chat_id":    n.cfg.TelegramChatID,
		"text":       FormatHTML(alarm, n.cfg.Environment),
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBaseURL, n.cfg.TelegramBotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := doJSON(n.client, req); err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	return nil
}

func doJSON(client *http.Client, req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
