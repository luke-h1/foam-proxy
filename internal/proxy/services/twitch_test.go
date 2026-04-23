package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeMetricsRecorder struct {
	calls []twitchMetricCall
}

type twitchMetricCall struct {
	operation  string
	app        string
	outcome    string
	statusCode int
	duration   time.Duration
}

func (f *fakeMetricsRecorder) RecordTwitchRequest(_ context.Context, operation, app, outcome string, statusCode int, duration time.Duration) {
	f.calls = append(f.calls, twitchMetricCall{
		operation:  operation,
		app:        app,
		outcome:    outcome,
		statusCode: statusCode,
		duration:   duration,
	})
}

func TestNewTwitchServiceUsesProvidedTimeout(t *testing.T) {
	svc := NewTwitchService("foam-app", "id", "secret", 3*time.Second, nil)

	if got, want := svc.httpClient.Timeout, 3*time.Second; got != want {
		t.Fatalf("httpClient.Timeout = %v, want %v", got, want)
	}
}

func TestDefaultTokenRecordsSuccessMetrics(t *testing.T) {
	recorder := &fakeMetricsRecorder{}
	svc := NewTwitchService("foam-app", "id", "secret", time.Second, recorder)
	svc.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(
					`{"access_token":"abc","expires_in":3600,"token_type":"bearer","scope":[]}`,
				)),
			}, nil
		}),
		Timeout: time.Second,
	}

	_, err := svc.DefaultToken(context.Background())
	if err != nil {
		t.Fatalf("DefaultToken() error = %v", err)
	}

	if got, want := len(recorder.calls), 1; got != want {
		t.Fatalf("len(recorder.calls) = %d, want %d", got, want)
	}

	call := recorder.calls[0]
	if call.operation != "default_token" || call.app != "foam-app" || call.outcome != "success" || call.statusCode != http.StatusOK {
		t.Fatalf("metric call = %+v", call)
	}
}

func TestRefreshTokenRecordsErrorMetrics(t *testing.T) {
	recorder := &fakeMetricsRecorder{}
	svc := NewTwitchService("foam-menubar", "id", "secret", time.Second, recorder)
	svc.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"status":400,"message":"bad refresh"}`)),
			}, nil
		}),
		Timeout: time.Second,
	}

	_, err := svc.RefreshToken(context.Background(), "bad-token")
	if err == nil {
		t.Fatal("RefreshToken() error = nil, want error")
	}

	if got, want := len(recorder.calls), 1; got != want {
		t.Fatalf("len(recorder.calls) = %d, want %d", got, want)
	}

	call := recorder.calls[0]
	if call.operation != "refresh_token" || call.app != "foam-menubar" || call.outcome != "error" || call.statusCode != http.StatusBadRequest {
		t.Fatalf("metric call = %+v", call)
	}
}

func TestDefaultTokenRecordsTransportErrorMetrics(t *testing.T) {
	recorder := &fakeMetricsRecorder{}
	svc := NewTwitchService("foam-app", "id", "secret", time.Second, recorder)
	svc.httpClient = &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport failed")
		}),
		Timeout: time.Second,
	}

	_, err := svc.DefaultToken(context.Background())
	if err == nil {
		t.Fatal("DefaultToken() error = nil, want error")
	}

	if got, want := len(recorder.calls), 1; got != want {
		t.Fatalf("len(recorder.calls) = %d, want %d", got, want)
	}

	call := recorder.calls[0]
	if call.operation != "default_token" || call.outcome != "error" || call.statusCode != 0 {
		t.Fatalf("metric call = %+v", call)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
