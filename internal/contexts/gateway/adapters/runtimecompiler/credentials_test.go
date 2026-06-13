package runtimecompiler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gatewayhttp "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/http/openai"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

func TestCompileGatewayRuntimeResolvesProviderCredentialRef(t *testing.T) {
	resolver := gatewayhttp.CredentialResolverFunc(func(ref string) (string, error) {
		if ref != "env:OPENAI_API_KEY" {
			t.Fatalf("credential ref = %q, want env:OPENAI_API_KEY", ref)
		}
		return "resolved-secret", nil
	})

	compiled, err := CompileGatewayRuntime(RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:           "openai",
			BaseURL:       "https://upstream.example",
			CredentialRef: "env:OPENAI_API_KEY",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        "POST",
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}, CompileOptions{CredentialResolver: resolver})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}

	if got := compiledGatewayConfig(t, compiled).Providers[0].APIKey; got != "resolved-secret" {
		t.Fatalf("compiled provider API key = %q, want resolved secret", got)
	}
}

func TestCompiledRuntimeGatewayUsesResolvedCredential(t *testing.T) {
	var gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]string{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	compiled, err := CompileGatewayRuntime(RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:           "openai",
			BaseURL:       upstream.URL,
			CredentialRef: "secret/openai",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        "POST",
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}, CompileOptions{
		CredentialResolver: gatewayhttp.CredentialResolverFunc(func(ref string) (string, error) {
			return "resolved-secret", nil
		}),
	})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}

	handler := gatewayhttp.NewServer(compiledGatewayConfig(t, compiled))
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if gotAuthorization != "Bearer resolved-secret" {
		t.Fatalf("upstream Authorization = %q, want resolved bearer token", gotAuthorization)
	}
}

func TestCompileGatewayRuntimeResolvesVerdictProviderCredentialRef(t *testing.T) {
	var gotAuthorization string
	verdictServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(verdict.Result{
			SuggestedAction: verdict.ActionAllow,
			RiskLevel:       verdict.RiskLow,
			ProviderName:    "sec-model",
		})
	}))
	defer verdictServer.Close()

	compiled, err := CompileGatewayRuntime(RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        "POST",
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		VerdictProviders: []VerdictProviderConfig{{
			Key:           "sec-model",
			Name:          "Security Model",
			Endpoint:      verdictServer.URL,
			CredentialRef: "secret/verdict",
			Enabled:       true,
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}, CompileOptions{
		CredentialResolver: gatewayhttp.CredentialResolverFunc(func(ref string) (string, error) {
			if ref != "secret/verdict" {
				t.Fatalf("credential ref = %q, want secret/verdict", ref)
			}
			return "verdict-secret", nil
		}),
	})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}
	if compiledGatewayConfig(t, compiled).VerdictProvider == nil {
		t.Fatal("compiled verdict provider is nil")
	}

	_, err = compiledGatewayConfig(t, compiled).VerdictProvider.Evaluate(t.Context(), verdict.Request{
		RouteKey:  "chat",
		Direction: verdict.DirectionInput,
		Text:      "hello",
	})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if gotAuthorization != "Bearer verdict-secret" {
		t.Fatalf("verdict Authorization = %q, want resolved bearer token", gotAuthorization)
	}
}

func TestCompileGatewayRuntimeRejectsUnresolvedProviderCredentialRef(t *testing.T) {
	wantErr := errors.New("missing credential")
	resolver := gatewayhttp.CredentialResolverFunc(func(ref string) (string, error) {
		return "", wantErr
	})

	_, err := CompileGatewayRuntime(RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:           "openai",
			BaseURL:       "https://upstream.example",
			CredentialRef: "env:OPENAI_API_KEY",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}, CompileOptions{CredentialResolver: resolver})
	if err == nil {
		t.Fatal("CompileGatewayRuntime() error = nil, want credential error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("CompileGatewayRuntime() error = %v, want wrapped missing credential", err)
	}
	if strings.Contains(err.Error(), "env:OPENAI_API_KEY") {
		t.Fatalf("CompileGatewayRuntime() error leaked credential ref: %v", err)
	}
}
