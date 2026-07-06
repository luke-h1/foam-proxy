package proxy

import (
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/foam/proxy/internal/config"
)

type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

func DefaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type":                 "application/json",
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET,OPTIONS,POST,PUT,DELETE",
	}
}

func (p *ProxyRequests) jsonResponse(statusCode int, body interface{}) Response {
	raw, err := json.Marshal(body)
	if err != nil {
		raw = []byte(`{"error":"internal server error"}`)
	}
	return Response{
		StatusCode: statusCode,
		Headers:    DefaultHeaders(),
		Body:       string(raw),
	}
}

func noStore(resp Response) Response {
	resp.Headers["Cache-Control"] = "no-store"
	return resp
}

func htmlResponse(statusCode int, body string) Response {
	headers := DefaultHeaders()
	headers["Content-Type"] = "text/html"
	return Response{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
	}
}

func magicTokenType(magic *config.MagicLink) string {
	if magic.TokenType == "" {
		return "bearer"
	}
	return magic.TokenType
}

func magicTokenBlob(magic *config.MagicLink) map[string]interface{} {
	blob := map[string]interface{}{
		"access_token": magic.AccessToken,
		"token_type":   magicTokenType(magic),
	}
	if magic.RefreshToken != "" {
		blob["refresh_token"] = magic.RefreshToken
	}
	if magic.ExpiresIn > 0 {
		blob["expires_in"] = magic.ExpiresIn
	}
	return blob
}

func magicTargetURL(magic *config.MagicLink, scheme string) string {
	form := url.Values{}
	form.Set("access_token", magic.AccessToken)
	if magic.RefreshToken != "" {
		form.Set("refresh_token", magic.RefreshToken)
	}
	form.Set("token_type", magicTokenType(magic))

	if magic.ExpiresIn > 0 {
		form.Set("expires_in", strconv.Itoa(magic.ExpiresIn))
	}

	return scheme + "://auth?" + form.Encode()
}
