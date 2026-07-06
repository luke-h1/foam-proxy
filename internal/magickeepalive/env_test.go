package magickeepalive

import (
	"context"
	"strings"
	"testing"
)

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestNewFromEnvRequiresSSMParam(t *testing.T) {
	_, err := NewFromEnv(context.Background(), fakeGetenv(map[string]string{
		"TWITCH_CLIENT_ID":     "id",
		"TWITCH_CLIENT_SECRET": "secret",
	}))
	if err == nil || !strings.Contains(err.Error(), "MAGIC_LINK_SSM_PARAM") {
		t.Fatalf("err = %v, want MAGIC_LINK_SSM_PARAM required error", err)
	}
}

func TestNewFromEnvRequiresTwitchCredentials(t *testing.T) {
	_, err := NewFromEnv(context.Background(), fakeGetenv(map[string]string{
		"MAGIC_LINK_SSM_PARAM": "/foo",
	}))
	if err == nil || !strings.Contains(err.Error(), "TWITCH_CLIENT_ID") {
		t.Fatalf("err = %v, want TWITCH_CLIENT_ID/SECRET required error", err)
	}
}
