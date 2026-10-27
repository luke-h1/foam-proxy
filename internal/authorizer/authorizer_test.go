package authorizer

import (
	"context"
	"testing"
)

func TestNewHandlerDeniesWhenAPIKeyUnset(t *testing.T) {
	t.Setenv("API_KEY", "")

	h := NewHandler()
	if h.hasKey {
		t.Fatal("hasKey = true, want false")
	}

	resp, err := h.HandleRequest(context.Background(), RequestEvent{
		Headers: map[string]string{"x-api-key": "secret"},
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if resp.IsAuthorized {
		t.Fatal("IsAuthorized = true, want false")
	}
}

func TestHandleRequest(t *testing.T) {
	t.Setenv("API_KEY", "secret")
	h := NewHandler()

	tests := []struct {
		name  string
		event RequestEvent
		want  bool
	}{
		{
			name:  "header key",
			event: RequestEvent{Headers: map[string]string{"X-API-Key": "secret"}},
			want:  true,
		},
		{
			name:  "query key is ignored",
			event: RequestEvent{QueryStringParameters: map[string]string{"apiKey": "secret"}},
			want:  false,
		},
		{
			name:  "wrong key",
			event: RequestEvent{Headers: map[string]string{"x-api-key": "nope"}},
			want:  false,
		},
		{
			name:  "missing key",
			event: RequestEvent{},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := h.HandleRequest(context.Background(), tt.event)
			if err != nil {
				t.Fatalf("HandleRequest() error = %v", err)
			}
			if resp.IsAuthorized != tt.want {
				t.Fatalf("IsAuthorized = %v, want %v", resp.IsAuthorized, tt.want)
			}
		})
	}
}
