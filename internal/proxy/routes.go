package proxy

import "encoding/json"

func (handlers *Handlers) Route(path, requestURL string) (statusCode int, headers map[string]string, body string) {
	headers = DefaultHeaders()

	switch path {
	case "/api/pending":
		headers["Content-Type"] = "text/html"
		return 200, headers, handlers.Pending()

	case "/api/proxy":
		redirectURI, err := RedirectURI(requestURL)
		if err != nil {
			b, _ := json.Marshal(map[string]interface{}{"error": "invalid request URL"})
			return 500, headers, string(b)
		}
		headers["Location"] = redirectURI
		return 302, headers, handlers.Proxy()

	case "/api/token":
		return 200, headers, handlers.Token()

	case "/api/healthcheck":
		return 200, headers, handlers.Health()

	case "/api/version":
		return 200, headers, handlers.Version()

	default:
		b, _ := json.Marshal(map[string]string{"error": "not found"})
		return 404, headers, string(b)
	}
}
