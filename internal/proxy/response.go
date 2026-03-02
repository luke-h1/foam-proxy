package proxy

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
