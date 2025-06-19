package httpclient

import (
	"net/http"
)

// HTTPClient defines the interface for making HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var client HTTPClient = http.DefaultClient

// SetClient overrides the HTTP client (e.g., for testing).
func SetClient(c HTTPClient) {
	client = c
}

// Do sends an HTTP request using the configured client.
func Do(req *http.Request) (*http.Response, error) {
	return client.Do(req)
}
