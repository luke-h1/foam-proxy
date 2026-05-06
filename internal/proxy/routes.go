package proxy

import (
	"context"
)

func (handlers *Handlers) Route(ctx context.Context, path, requestURL string, query map[string]string) (statusCode int, headers map[string]string, body string) {
	headers = DefaultHeaders()

	app := ""

	if query != nil {
		app = query["app"]
	}

	switch path {
	case "/api/pending":
		headers["Content-Type"] = "text/html"
		return 200, headers, handlers.Pending()

	case "/api/proxy":
		redirectURI, err := handlers.RedirectURI(app, requestURL)
		if err != nil {
			return 400, headers, clientErrorBody(err)
		}
		headers["Location"] = redirectURI
		return 302, headers, handlers.Proxy()

	case "/api/token":
		body, err := handlers.Token(ctx, app)

		if err != nil {
			return 400, headers, body
		}
		return 200, headers, body

	case "/api/refresh-token":
		refreshToken := ""
		if query != nil {
			refreshToken = query["token"]
		}
		body, err := handlers.RefreshToken(ctx, app, refreshToken)

		if err != nil {
			return 400, headers, body
		}

		return 200, headers, body

	case "/api/healthcheck":
		return 200, headers, handlers.Health()

	case "/api/version":
		return 200, headers, handlers.Version(ctx)

	default:
		return 404, headers, errorBody(errNotFound)
	}
}
