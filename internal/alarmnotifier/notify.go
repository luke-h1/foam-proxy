package alarmnotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	discordContentLimit = 2000
	discordEmbedLimit   = 4096
	telegramTextLimit   = 4096
	httpTimeout         = 5 * time.Second
)

var (
	httpClient = &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
	type result struct {
		name string
		err  error
	}

	var (
		wg      sync.WaitGroup
		results = make(chan result, 2)
		sent    atomic.Int32
	)

	if n.cfg.HasDiscord() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.sendDiscord(ctx, alarm); err != nil {
				results <- result{name: "discord", err: err}
				return
			}
			sent.Add(1)
		}()
	}

	if n.cfg.HasTelegram() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := n.sendTelegram(ctx, alarm); err != nil {
				results <- result{name: "telegram", err: err}
				return
			}
			sent.Add(1)
		}()
	}

	wg.Wait()
	close(results)

	var errs []string
	for r := range results {
		errs = append(errs, fmt.Sprintf("%s: %v", r.name, r.err))
	}

	if len(errs) == 0 {
		return nil
	}

	// At least one channel delivered — log the rest but do not fail the invoke,
	// otherwise SNS/Lambda retries re-send to the channel that already succeeded.
	if sent.Load() > 0 {
		log.Printf("partial notify failure: %s", strings.Join(errs, "; "))
		return nil
	}

	return fmt.Errorf("notify failed: %s", strings.Join(errs, "; "))
}

func (n *Notifier) sendDiscord(ctx context.Context, alarm *AlarmNotification) error {
	payload := map[string]any{
		"content": truncate(fmt.Sprintf("CW alarm **%s** -> `%s`", alarm.AlarmName, alarm.NewStateValue), discordContentLimit),
		"embeds": []map[string]any{
			{
				"title":       truncate(fmt.Sprintf("CW: %s", alarm.NewStateValue), 256),
				"description": truncate(FormatPlainText(alarm, n.cfg.Environment), discordEmbedLimit),
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
		"text":       truncate(FormatHTML(alarm, n.cfg.Environment), telegramTextLimit),
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

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
