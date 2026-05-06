package proxy

import "encoding/json"

func (handlers *Handlers) Route(path, requestURL string, query map[string]string) (statusCode int, headers map[string]string, body string) {
	headers = DefaultHeaders()

	switch path {
	case "/api/pending":
		headers["Content-Type"] = "text/html"
		return 200, headers, handlers.Pending()

	case "/api/proxy":
		headers["Content-Type"] = "text/html"
		return 200, headers, handlers.Proxy()

	case "/api/token":
		return 200, headers, handlers.Token()

	case "/api/refresh-token":
		refreshToken := ""
		if query != nil {
			refreshToken = query["token"]
		}
		respBody := handlers.RefreshToken(refreshToken)
		if refreshToken == "" {
			return 400, headers, respBody
		}
		return 200, headers, respBody

	case "/api/healthcheck":
		return 200, headers, handlers.Health()

	case "/api/version":
		return 200, headers, handlers.Version()

	default:
		b, _ := json.Marshal(map[string]string{"error": "not found"})
		return 404, headers, string(b)
	}
}
