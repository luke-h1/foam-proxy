package authorizer

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
)

type RequestEvent struct {
	Type                  string                 `json:"type"`
	MethodArn             string                 `json:"methodArn"`
	Headers               map[string]interface{} `json:"headers"`
	QueryStringParameters map[string]interface{} `json:"queryStringParameters"`
	IdentitySource        string                 `json:"identitySource"`
}

type Handler struct {
	expectedAPIKey string
}

func NewHandler() *Handler {
	key := os.Getenv("API_KEY")
	if key != "" {
		return &Handler{
			expectedAPIKey: key,
		}
	}
	log.Print("API_KEY not set")
	return &Handler{}
}

func (h *Handler) HandleRequest(ctx context.Context, input RequestEvent) (events.APIGatewayCustomAuthorizerResponse, error) {
	methodArn := input.MethodArn

	if methodArn == "" {
		methodArn = "*"
	}

	deny := func() (events.APIGatewayCustomAuthorizerResponse, error) {
		return generatePolicy("user", "Deny", methodArn), nil
	}

	if h.expectedAPIKey == "" {
		return deny()
	}

	headers := normalizeMap(input.Headers)
	query := normalizeMap(input.Headers)

	apiKey := headers["x-api-key"]

	if apiKey == "" {
		apiKey = query["apikey"]
	}

	if apiKey == "" {
		apiKey = input.IdentitySource
	}

	if !constantTimeEquals(apiKey, h.expectedAPIKey) {
		log.Printf(`{"level":"info","msg":"authorizer deny","reason":"invalid_or_missing_key"}`)
		sentry.NewMeter(ctx).Count("authorizer.deny", 1,
			sentry.WithAttributes(attribute.String("reason", "invalid_or_missing_key")),
		)
		return deny()
	}
	return generatePolicy("user", "Allow", methodArn), nil
}

func generatePolicy(principalID, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
	return events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: principalID,
		PolicyDocument: events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   effect,
					Resource: []string{resource},
				},
			},
		},
	}
}

func normalizeMap(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}

	out := make(map[string]string, len(m))

	for k, v := range m {
		if s, ok := v.(string); ok {
			out[strings.ToLower(k)] = s
		}
	}
	return out
}

func constantTimeEquals(a, b string) bool {
	if len(a) != len(b) {
		// Still run loop over max length so timing doesn't leak length.
		n := len(a)
		if len(b) > n {
			n = len(b)
		}
		var result int
		for i := 0; i < n; i++ {
			var ac, bc byte
			if i < len(a) {
				ac = a[i]
			}
			if i < len(b) {
				bc = b[i]
			}
			result |= int(ac ^ bc)
		}
		return false
	}
	var result int
	for i := 0; i < len(a); i++ {
		result |= int(a[i] ^ b[i])
	}
	return result == 0
}

func InitSentry() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return
	}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      os.Getenv("SENTRY_ENVIRONMENT"),
		Release:          os.Getenv("SENTRY_RELEASE"),
		TracesSampleRate: 0.5,
	})
	if err != nil {
		log.Printf("sentry init: %v", err)
	}
}
