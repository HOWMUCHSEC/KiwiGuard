package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddlewareRecoversRequestPanic(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{})
	handler := factory.httpMiddleware("gateway")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("request boom")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Body.String(); got != "Internal Server Error\n" {
		t.Fatalf("body = %q, want generic internal server error", got)
	}
}

func TestHTTPMiddlewareDoesNotAppendErrorAfterResponseCommitted(t *testing.T) {
	factory := NewFactoryWithOptions(FactoryOptions{})
	handler := factory.httpMiddleware("gateway")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial response"))
		panic("request boom")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if got := rec.Body.String(); got != "partial response" {
		t.Fatalf("body = %q, want only committed response body", got)
	}
}
