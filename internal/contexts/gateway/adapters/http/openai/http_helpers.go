package openai

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// readLimited reads at most max bytes and reports whether the limit was exceeded.
func readLimited(r io.Reader, max int64) ([]byte, bool, error) {
	limited := io.LimitReader(r, max+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > max {
		return nil, true, nil
	}
	return body, false, nil
}

// endpointKind maps supported API paths to stable observability labels.
func endpointKind(path string) string {
	switch path {
	case chatCompletionsPath:
		return "chat_completions"
	case responsesPath:
		return "responses"
	default:
		return "unknown"
	}
}

// joinURL appends path to a validated upstream base URL.
func joinURL(base string, path string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse upstream url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("parse upstream url: base url is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	return parsed.String(), nil
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Content-Length") || hopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isEventStream(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(contentType), "text/event-stream")
}

func writeSSETermination(w io.Writer, reason string) {
	_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":{\"code\":%q,\"message\":\"stream terminated by gateway\"}}\n\n", reason)
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func hopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "proxy-connection", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func clientKeyFromAuthorization(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(value, " ")
	if !ok {
		return value
	}
	if !strings.EqualFold(scheme, "Bearer") {
		return value
	}
	return strings.TrimSpace(token)
}

func authStatus(reason string) int {
	switch reason {
	case "missing_client_key", "invalid_client_key":
		return http.StatusUnauthorized
	case "disabled_client_key", "revoked_client_key":
		return http.StatusForbidden
	default:
		return http.StatusUnauthorized
	}
}

func requestIDFromHeader(r *http.Request) string {
	for _, key := range []string{"X-Request-ID", "OpenAI-Request-ID"} {
		if value := r.Header.Get(key); value != "" {
			return value
		}
	}
	return randomID()
}

func randomID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
