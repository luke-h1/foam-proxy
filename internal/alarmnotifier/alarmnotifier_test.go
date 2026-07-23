package alarmnotifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestLoadConfigRequiresDestination(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when no destinations configured")
	}
}

func TestLoadConfigTelegramPair(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "")
	t.Setenv("TELEGRAM_BOT_TOKEN", "token")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when only telegram token set")
	}
}

func TestLoadConfigDiscordOnly(t *testing.T) {
	t.Setenv("DISCORD_WEBHOOK_URL", "https://discord.example/webhook")
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasDiscord() || cfg.HasTelegram() {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestParseAlarmMessage(t *testing.T) {
	raw := `{
		"AlarmName": "foam-proxy-lambda-staging-invocations-anomaly",
		"AlarmDescription": "Unusual invocation rate",
		"NewStateValue": "ALARM",
		"NewStateReason": "Threshold Crossed",
		"OldStateValue": "OK",
		"StateChangeTime": "2026-07-23T13:00:00.000+0000",
		"Region": "eu-west-2"
	}`
	alarm, err := ParseAlarmMessage(raw)
	if err != nil {
		t.Fatal(err)
	}
	if alarm.AlarmName == "" || alarm.NewStateValue != "ALARM" {
		t.Fatalf("unexpected alarm: %+v", alarm)
	}
}

func TestFormatHTMLEscapes(t *testing.T) {
	alarm := &AlarmNotification{
		AlarmName:      "a<b>",
		NewStateValue:  "ALARM",
		NewStateReason: "x & y",
	}
	got := FormatHTML(alarm, "staging")
	if !strings.Contains(got, "a&lt;b&gt;") {
		t.Fatalf("expected escaped alarm name, got %q", got)
	}
	if !strings.Contains(got, "x &amp; y") {
		t.Fatalf("expected escaped reason, got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	got := truncate(strings.Repeat("a", 10), 5)
	if got != "aaaa…" {
		t.Fatalf("got %q", got)
	}
	if truncate("short", 10) != "short" {
		t.Fatal("should not truncate short strings")
	}
}

func TestNotifyPostsBothChannels(t *testing.T) {
	var discordHits, telegramHits atomic.Int32
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		discordHits.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "ALARM") {
			t.Errorf("discord body missing ALARM: %s", body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		telegramHits.Add(1)
		if !strings.HasSuffix(r.URL.Path, "/bottest-token/sendMessage") {
			t.Errorf("unexpected telegram path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer telegram.Close()

	prevBase := telegramAPIBaseURL
	telegramAPIBaseURL = telegram.URL
	t.Cleanup(func() { telegramAPIBaseURL = prevBase })

	n := NewNotifier(&Config{
		DiscordWebhookURL: discord.URL,
		TelegramBotToken:  "test-token",
		TelegramChatID:    "123",
		Environment:       "staging",
	})

	alarm := &AlarmNotification{
		AlarmName:      "test-alarm",
		NewStateValue:  "ALARM",
		NewStateReason: "spike",
	}
	if err := n.Notify(context.Background(), alarm); err != nil {
		t.Fatal(err)
	}
	if discordHits.Load() != 1 || telegramHits.Load() != 1 {
		t.Fatalf("discord=%d telegram=%d", discordHits.Load(), telegramHits.Load())
	}
}

func TestNotifyPartialFailureDoesNotError(t *testing.T) {
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer telegram.Close()

	prevBase := telegramAPIBaseURL
	telegramAPIBaseURL = telegram.URL
	t.Cleanup(func() { telegramAPIBaseURL = prevBase })

	n := NewNotifier(&Config{
		DiscordWebhookURL: discord.URL,
		TelegramBotToken:  "test-token",
		TelegramChatID:    "123",
	})
	err := n.Notify(context.Background(), &AlarmNotification{AlarmName: "a", NewStateValue: "ALARM"})
	if err != nil {
		t.Fatalf("expected nil when at least one channel succeeds, got %v", err)
	}
}

func TestNotifyAllChannelsFail(t *testing.T) {
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer discord.Close()

	telegram := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer telegram.Close()

	prevBase := telegramAPIBaseURL
	telegramAPIBaseURL = telegram.URL
	t.Cleanup(func() { telegramAPIBaseURL = prevBase })

	n := NewNotifier(&Config{
		DiscordWebhookURL: discord.URL,
		TelegramBotToken:  "test-token",
		TelegramChatID:    "123",
	})
	if err := n.Notify(context.Background(), &AlarmNotification{AlarmName: "a", NewStateValue: "ALARM"}); err == nil {
		t.Fatal("expected error when all channels fail")
	}
}

func TestDoJSONRejectsRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not follow redirect")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer final.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer redirect.Close()

	req, err := http.NewRequest(http.MethodPost, redirect.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = doJSON(httpClient, req)
	if err == nil {
		t.Fatal("expected error on redirect response")
	}
}

func TestHandleSNS(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	n := NewNotifier(&Config{DiscordWebhookURL: srv.URL, Environment: "staging"})

	msg, _ := json.Marshal(AlarmNotification{
		AlarmName:     "foam-proxy-lambda-staging-invocations-anomaly",
		NewStateValue: "ALARM",
	})
	err := n.HandleSNS(context.Background(), events.SNSEvent{
		Records: []events.SNSEventRecord{{
			SNS: events.SNSEntity{Message: string(msg)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 notify, got %d", hits)
	}
}
