package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefreshTokenSendsFormEncodedRequest(t *testing.T) {
	var gotContentType, gotGrantType, gotRefreshToken, gotClientID, gotClientSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm() error = %v", err)
		}
		gotGrantType = r.PostForm.Get("grant_type")
		gotRefreshToken = r.PostForm.Get("refresh_token")
		gotClientID = r.PostForm.Get("client_id")
		gotClientSecret = r.PostForm.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"access_token":"NEW","refresh_token":"NEWREF","expires_in":14033,"token_type":"bearer"}`)); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}))
	defer srv.Close()

	svc := NewTwitchService("client-id", "client-secret", time.Second)
	svc.baseURL = srv.URL

	out, err := svc.RefreshToken(context.Background(), "the-refresh-token")
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}

	// Twitch's token endpoint requires the form-encoded Content-Type; without it
	// the body is unparsed and Twitch answers 400 "Invalid refresh token".
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", gotContentType)
	}
	if gotGrantType != "refresh_token" {
		t.Errorf("grant_type = %q, want refresh_token", gotGrantType)
	}
	if gotRefreshToken != "the-refresh-token" {
		t.Errorf("refresh_token = %q, want the-refresh-token", gotRefreshToken)
	}
	if gotClientID != "client-id" || gotClientSecret != "client-secret" {
		t.Errorf("client_id/secret = %q/%q, want client-id/client-secret", gotClientID, gotClientSecret)
	}
	if out.AccessToken != "NEW" || out.RefreshToken != "NEWREF" || out.ExpiresIn != 14033 {
		t.Errorf("response = %+v, want NEW/NEWREF/14033", out)
	}
}

func TestRefreshTokenReturnsUpstreamErrorOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"status":400,"message":"Invalid refresh token"}`)); err != nil {
			t.Errorf("Write() error = %v", err)
		}
	}))
	defer srv.Close()

	svc := NewTwitchService("client-id", "client-secret", time.Second)
	svc.baseURL = srv.URL

	_, err := svc.RefreshToken(context.Background(), "dead-token")
	if err == nil {
		t.Fatal("expected error on 400 response")
	}
	if err.Error() != "Invalid refresh token" {
		t.Errorf("error = %q, want \"Invalid refresh token\"", err.Error())
	}
}

func TestNewTwitchServiceUsesProvidedTimeout(t *testing.T) {
	svc := NewTwitchService("id", "secret", 7*time.Second)
	if svc.httpClient.Timeout != 7*time.Second {
		t.Errorf("httpClient.Timeout = %v, want 7s", svc.httpClient.Timeout)
	}

	zero := NewTwitchService("id", "secret", 0)
	if zero.httpClient.Timeout != defaultTimeout {
		t.Errorf("zero timeout = %v, want %v", zero.httpClient.Timeout, defaultTimeout)
	}
}

func TestDefaultTokenHonorsContext(t *testing.T) {
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-unblock:
		}
	}))
	t.Cleanup(func() {
		close(unblock)
		srv.Close()
	})

	svc := NewTwitchService("client-id", "client-secret", 5*time.Second)
	svc.baseURL = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := svc.DefaultToken(ctx)
	if err == nil {
		t.Fatal("expected context cancellation")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("DefaultToken returned in %s; request context was not honored", time.Since(start))
	}
}
