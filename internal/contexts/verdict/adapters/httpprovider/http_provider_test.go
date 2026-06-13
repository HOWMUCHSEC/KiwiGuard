package httpprovider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	domainverdict "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

func TestNewHTTPProviderDefaultsHTTPClient(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:     "default-client",
		Endpoint: "http://example.test/verdict",
	})

	httpProvider, ok := provider.(httpProvider)
	if !ok {
		t.Fatalf("provider type = %T, want httpProvider", provider)
	}
	if httpProvider.httpClient != http.DefaultClient {
		t.Fatal("httpClient did not default to http.DefaultClient")
	}
}

func TestHTTPProviderSendsStructuredVerdictRequest(t *testing.T) {
	var got httpProviderRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(httpProviderResult{
			RiskLevel:       domainverdict.RiskLow,
			Categories:      []string{"ok"},
			Confidence:      0.1,
			SuggestedAction: domainverdict.ActionAllow,
			MatchedSpans: []httpProviderMatchedSpan{{
				Start:    1,
				End:      4,
				Category: "ok",
				TextHash: "hash",
			}},
			Rationale:    "safe enough",
			ProviderName: "http-test",
		})
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:       "http-test",
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
		Timeout:    time.Second,
	})
	result, err := provider.Evaluate(context.Background(), domainverdict.Request{
		RequestID: "req-1",
		RouteKey:  "openai",
		Model:     "gpt-test",
		Direction: domainverdict.DirectionOutput,
		Text:      "safe",
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if got.RequestID != "req-1" || got.Direction != domainverdict.DirectionOutput {
		t.Fatalf("unexpected request: %+v", got)
	}
	if result.ProviderName != "http-test" {
		t.Fatalf("ProviderName = %q, want http-test", result.ProviderName)
	}
	if len(result.MatchedSpans) != 1 || result.MatchedSpans[0].TextHash != "hash" {
		t.Fatalf("MatchedSpans = %+v, want decoded HTTP provider span", result.MatchedSpans)
	}
	if result.Rationale != "safe enough" {
		t.Fatalf("Rationale = %q, want safe enough", result.Rationale)
	}
}

func TestHTTPProviderDefaultsProviderNameWhenResponseOmitsIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(httpProviderResult{
			RiskLevel:       domainverdict.RiskLow,
			Categories:      []string{"ok"},
			Confidence:      0.1,
			SuggestedAction: domainverdict.ActionAllow,
		})
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:       "configured-provider",
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	})
	result, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if result.ProviderName != "configured-provider" {
		t.Fatalf("ProviderName = %q, want configured-provider", result.ProviderName)
	}
	if result.Latency <= 0 {
		t.Fatalf("Latency = %v, want positive", result.Latency)
	}
}

func TestHTTPProviderReturnsErrorForNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:       "http-test",
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	})
	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want non-2xx error")
	}
}

func TestHTTPProviderReturnsErrorWhenEndpointIsMissing(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{Name: "missing-endpoint"})

	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want missing endpoint error")
	}
	if !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("Evaluate() error = %v, want endpoint is required", err)
	}
}

func TestHTTPProviderReturnsErrorForInvalidEndpoint(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:     "bad-endpoint",
		Endpoint: "://bad-url",
	})

	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want invalid endpoint error")
	}
	if !strings.Contains(err.Error(), "create verdict request") {
		t.Fatalf("Evaluate() error = %v, want create verdict request", err)
	}
}

func TestHTTPProviderReturnsErrorForTransportFailure(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:     "transport-error",
		Endpoint: "http://example.test/verdict",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("dial failed")
			}),
		},
	})

	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want transport error")
	}
	if !strings.Contains(err.Error(), "send verdict request") {
		t.Fatalf("Evaluate() error = %v, want send verdict request", err)
	}
}

func TestHTTPProviderReturnsErrorForInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{not-json"))
	}))
	defer server.Close()

	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:       "bad-json",
		Endpoint:   server.URL,
		HTTPClient: server.Client(),
	})
	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want decode error")
	}
	if !strings.Contains(err.Error(), "decode verdict response") {
		t.Fatalf("Evaluate() error = %v, want decode verdict response", err)
	}
}

func TestHTTPProviderAppliesTimeoutContextToRequest(t *testing.T) {
	provider := NewHTTPProvider(HTTPProviderOptions{
		Name:     "timeout-context",
		Endpoint: "http://example.test/verdict",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			}),
		},
		Timeout: time.Nanosecond,
	})

	_, err := provider.Evaluate(context.Background(), domainverdict.Request{Text: "safe"})
	if err == nil {
		t.Fatal("Evaluate() error = nil, want timeout error")
	}
	if !strings.Contains(err.Error(), "send verdict request") {
		t.Fatalf("Evaluate() error = %v, want send verdict request", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
