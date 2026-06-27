package magickeepalive

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/foam/proxy/internal/config"
	"github.com/foam/proxy/internal/proxy/services"
)

type fakeStore struct {
	get    string
	getErr error
	put    string
	putErr error
	puts   int
}

func (f *fakeStore) Get(context.Context) (string, error) { return f.get, f.getErr }
func (f *fakeStore) Put(_ context.Context, v string) error {
	f.puts++
	f.put = v
	return f.putErr
}

type fakeTwitch struct {
	resp     *services.TwitchRefreshTokenResponse
	err      error
	gotToken string
}

func (f *fakeTwitch) RefreshToken(token string) (*services.TwitchRefreshTokenResponse, error) {
	f.gotToken = token
	return f.resp, f.err
}

func TestRefreshRotatesAndPersists(t *testing.T) {
	store := &fakeStore{get: `{"access_token":"OLD","refresh_token":"REF","expires_in":14400,"token_type":"bearer"}`}
	twitch := &fakeTwitch{resp: &services.TwitchRefreshTokenResponse{
		AccessToken: "NEW", RefreshToken: "NEWREF", ExpiresIn: 13000, TokenType: "bearer",
	}}

	if err := New(store, twitch).Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if twitch.gotToken != "REF" {
		t.Fatalf("refreshed with %q, want REF", twitch.gotToken)
	}

	got := config.ParseMagicLink(store.put)
	if got == nil {
		t.Fatalf("persisted blob did not parse: %q", store.put)
	}
	if got.AccessToken != "NEW" || got.RefreshToken != "NEWREF" || got.ExpiresIn != 13000 {
		t.Fatalf("persisted blob = %+v", got)
	}
}

func TestRefreshKeepsOldRefreshTokenWhenOmitted(t *testing.T) {
	store := &fakeStore{get: `{"access_token":"OLD","refresh_token":"REF","token_type":"bearer"}`}
	twitch := &fakeTwitch{resp: &services.TwitchRefreshTokenResponse{AccessToken: "NEW"}}

	if err := New(store, twitch).Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	var got config.MagicLink
	if err := json.Unmarshal([]byte(store.put), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.RefreshToken != "REF" {
		t.Fatalf("refresh_token = %q, want carried-over REF", got.RefreshToken)
	}
	if got.TokenType != "bearer" {
		t.Fatalf("token_type = %q, want carried-over bearer", got.TokenType)
	}
}

func TestRefreshFailsWithoutStoredRefreshToken(t *testing.T) {
	store := &fakeStore{get: `{"access_token":"OLD"}`}
	twitch := &fakeTwitch{}

	if err := New(store, twitch).Refresh(context.Background()); err == nil {
		t.Fatal("expected error for blob without refresh token")
	}
	if store.puts != 0 {
		t.Fatalf("Put called %d times, want 0 when nothing to rotate", store.puts)
	}
}

func TestRefreshDoesNotPersistOnTwitchError(t *testing.T) {
	store := &fakeStore{get: `{"access_token":"OLD","refresh_token":"REF"}`}
	twitch := &fakeTwitch{err: errors.New("twitch down")}

	if err := New(store, twitch).Refresh(context.Background()); err == nil {
		t.Fatal("expected error when Twitch refresh fails")
	}
	if store.puts != 0 {
		t.Fatalf("Put called %d times, want 0 on Twitch failure", store.puts)
	}
}
