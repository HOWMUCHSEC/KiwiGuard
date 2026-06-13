package openai

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// providerConfig is the transport-ready upstream provider configuration for one exchange.
type providerConfig struct {
	provider Provider
	client   *http.Client
}

// forward sends the projected request to the selected upstream provider.
func (s *server) forward(ctx context.Context, provider providerConfig, original *http.Request, body []byte) (*http.Response, io.Reader, error) {
	upstreamURL, err := joinURL(provider.provider.BaseURL, original.URL.Path)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, original.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.provider.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+provider.provider.APIKey)
	}
	for key, values := range original.Header {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Content-Type") || hopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	for key, value := range provider.provider.Headers {
		req.Header.Set(key, value)
	}

	resp, err := provider.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send upstream request: %w", err)
	}
	return resp, resp.Body, nil
}

// withTimeout returns a derived context only when a positive timeout is configured.
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}
