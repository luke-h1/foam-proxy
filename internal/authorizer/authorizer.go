package authorizer

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/foam/proxy/internal/config"
	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

type RequestEvent struct {
	Version               string            `json:"version"`
	Type                  string            `json:"type"`
	RouteArn              string            `json:"routeArn"`
	Headers               map[string]string `json:"headers"`
	QueryStringParameters map[string]string `json:"queryStringParameters"`
	IdentitySource        json.RawMessage   `json:"identitySource"`
}

type Handler struct {
	expectedKeyHash [32]byte
	hasKey          bool
}

func NewHandler() *Handler {
	key := os.Getenv("API_KEY")
	if key == "" {
		slog.Warn("API_KEY is unset; denying every request")
		return &Handler{}
	}

	return &Handler{
		expectedKeyHash: sha256.Sum256([]byte(key)),
		hasKey:          true,
	}
}

func (h *Handler) HandleRequest(ctx context.Context, input RequestEvent) (events.APIGatewayV2CustomAuthorizerSimpleResponse, error) {
	deny := func(reason string) (events.APIGatewayV2CustomAuthorizerSimpleResponse, error) {
		if reason != "" {
			sentry.NewMeter(ctx).Count("authorizer.deny", 1, sentry.WithAttributes(attribute.String("reason", reason)))
		}
		return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: false}, nil
	}

	if !h.hasKey {
		return deny("denied")
	}

	apiKey := normalizeMap(input.Headers)["x-api-key"]
	if !h.keyMatches(apiKey) {
		slog.Info("authorizer deny", "reason", "invalid_or_missing_key")
		return deny("denied")
	}

	sentry.NewMeter(ctx).Count("authorizer.allow", 1)
	return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: true}, nil
}

func (h *Handler) keyMatches(provided string) bool {
	sum := sha256.Sum256([]byte(provided))
	return subtle.ConstantTimeCompare(sum[:], h.expectedKeyHash[:]) == 1
}

func normalizeMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}

	out := make(map[string]string, len(m))
	for k, v := range m {
		out[strings.ToLower(k)] = v
	}
	return out
}

func InitSentry() {
	config.InitSentry("AUTH_DSN")
}
