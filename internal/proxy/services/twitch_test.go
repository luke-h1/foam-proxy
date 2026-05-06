package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeMetricsRecorder struct {
	successCalls []twitchMetricCall
	failureCalls []twitchFailureCall
}

type twitchMetricCall struct {
	operation  string
	app        string
	outcome    string
	statusCode int
	duration   time.Duration
}

type twitchFailureCall struct {
	operation    string
	app          string
	reason       string
	statusCode   int
	duration     time.Duration
	responseBody string
	err          error
}

func (f *fakeMetricsRecorder) RecordTwitchSuccess(_ context.Context, operation, app string, statusCode int, duration time.Duration) {
	f.successCalls = append(f.successCalls, twitchMetricCall{
		operation:  operation,
		app:        app,
		outcome:    "success",
		statusCode: statusCode,
		duration:   duration,
	})
}

func (f *fakeMetricsRecorder) RecordTwitchFailure(_ context.Context, operation, app, reason string, statusCode int, duration time.Duration, responseBody string, err error) {
	f.failureCalls = append(f.failureCalls, twitchFailureCall{
		operation:    operation,
		app:          app,
		reason:       reason,
		statusCode:   statusCode,
		duration:     duration,
		responseBody: responseBody,
		err:          err,
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
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got := req.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
				t.Fatalf("Content-Type = %q", got)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}
			if got := values.Get("grant_type"); got != "client_credentials" {
				t.Fatalf("grant_type = %q", got)
			}
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

	if got, want := len(recorder.successCalls), 1; got != want {
		t.Fatalf("len(recorder.successCalls) = %d, want %d", got, want)
	}

	call := recorder.successCalls[0]
	if call.operation != "default_token" || call.app != "foam-app" || call.outcome != "success" || call.statusCode != http.StatusOK {
		t.Fatalf("metric call = %+v", call)
	}
}

func TestRefreshTokenRecordsErrorMetrics(t *testing.T) {
	recorder := &fakeMetricsRecorder{}
	svc := NewTwitchService("foam-menubar", "id", "secret", time.Second, recorder)
	svc.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			values, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("ParseQuery() error = %v", err)
			}
			if got := values.Get("grant_type"); got != "refresh_token" {
				t.Fatalf("grant_type = %q", got)
			}
			if got := values.Get("refresh_token"); got != "bad-token" {
				t.Fatalf("refresh_token = %q", got)
			}
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

	if got, want := len(recorder.failureCalls), 1; got != want {
		t.Fatalf("len(recorder.failureCalls) = %d, want %d", got, want)
	}

	call := recorder.failureCalls[0]
	if call.operation != "refresh_token" || call.app != "foam-menubar" || call.reason != "bad_status" || call.statusCode != http.StatusBadRequest {
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

	if got, want := len(recorder.failureCalls), 1; got != want {
		t.Fatalf("len(recorder.failureCalls) = %d, want %d", got, want)
	}

	call := recorder.failureCalls[0]
	if call.operation != "default_token" || call.reason != "request_failed" || call.statusCode != 0 {
		t.Fatalf("metric call = %+v", call)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
