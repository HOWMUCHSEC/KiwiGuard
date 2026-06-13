package openai

import (
	"errors"
	"testing"
	"time"
)

func TestBuildConfigCompilesRuntimeConfig(t *testing.T) {
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 42},
		Routes: []RouteConfig{{
			Key:            "chat",
			Method:         "POST",
			Path:           "/v1/chat/completions",
			ProviderKey:    "openai",
			RequestedModel: "gpt-4.1",
			MappedModel:    "gpt-4.1-mini",
			UpstreamModel:  "gpt-4.1-mini",
			ExecutionMode:  string(ExecutionInline),
			FallbackAction: string(ActionAllow),
		}},
		Providers: []ProviderConfig{{
			Key:           "openai",
			BaseURL:       "https://api.openai.test",
			CredentialRef: "openai-key",
			Headers:       map[string]string{"x-tenant": "kiwi"},
			Timeout:       3 * time.Second,
		}},
		ModelMappings: []ModelMappingConfig{{
			Key:            "chat-map",
			RouteKey:       "chat",
			ProviderKey:    "openai",
			RequestedModel: "gpt-4.1",
			MappedModel:    "gpt-4.1-mini",
			UpstreamModel:  "gpt-4.1-mini",
		}},
		VerdictProviders: []VerdictProviderConfig{{
			Key:      "guard",
			Name:     "guard",
			Endpoint: "http://guard.test/evaluate",
			Enabled:  true,
			Timeout:  250 * time.Millisecond,
		}},
		RouteVerdictProviderBindings: []RouteVerdictProviderBindingConfig{{
			RouteKey:           "chat",
			VerdictProviderKey: "guard",
			ExecutionMode:      string(ExecutionAsyncShadow),
			Priority:           10,
		}},
		RawCapture: []RawCaptureConfig{{
			ID:            "capture-chat",
			RouteKey:      "chat",
			Direction:     "both",
			Enabled:       true,
			SampleRate:    0.5,
			RedactionMode: "hash",
		}},
		GatewayClients: []GatewayClientConfig{{
			ID:        "client-id",
			Name:      "console",
			Status:    string(ClientStatusEnabled),
			KeyPrefix: "kg_",
			KeyHash:   "hash",
		}},
		RouteLimits: []RouteLimitConfig{{
			RouteKey:              "chat",
			RequestsPerWindow:     100,
			Window:                time.Minute,
			MaxConcurrentRequests: 8,
			MaxBodyBytes:          4096,
		}},
		ClientRouteLimitOverrides: []ClientRouteLimitOverrideConfig{{
			ClientID:              "client-id",
			RouteKey:              "chat",
			RequestsPerWindow:     10,
			Window:                time.Minute,
			MaxConcurrentRequests: 2,
			MaxBodyBytes:          1024,
		}},
	}
	resolver := CredentialResolverFunc(func(ref string) (string, error) {
		if ref != "openai-key" {
			return "", ErrCredentialNotFound
		}
		return "resolved-key", nil
	})

	cfg, err := BuildConfig(input, CompileOptions{
		MaxBodyBytes:       8192,
		UpstreamTimeout:    5 * time.Second,
		VerdictTimeout:     time.Second,
		CredentialResolver: resolver,
	}, nil)
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}

	if cfg.ConfigRevisionNumber != 42 {
		t.Fatalf("revision number = %d, want 42", cfg.ConfigRevisionNumber)
	}
	if len(cfg.Routes) != 1 {
		t.Fatalf("routes length = %d, want 1", len(cfg.Routes))
	}
	route := cfg.Routes[0]
	if route.ProviderKey != "openai" || route.VerdictProviderKey != "guard" {
		t.Fatalf("route providers = (%q, %q), want (openai, guard)", route.ProviderKey, route.VerdictProviderKey)
	}
	if route.Execution != ExecutionAsyncShadow {
		t.Fatalf("route execution = %q, want %q", route.Execution, ExecutionAsyncShadow)
	}
	if !route.RequireClientAuth {
		t.Fatal("route RequireClientAuth = false, want true")
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].APIKey != "resolved-key" {
		t.Fatalf("provider credential was not resolved: %#v", cfg.Providers)
	}
	if cfg.Providers[0].Headers["x-tenant"] != "kiwi" {
		t.Fatalf("provider header missing: %#v", cfg.Providers[0].Headers)
	}
	input.Providers[0].Headers["x-tenant"] = "mutated"
	if cfg.Providers[0].Headers["x-tenant"] != "kiwi" {
		t.Fatal("provider headers were not cloned")
	}
	if len(cfg.RawCapturePolicies) != 1 || !cfg.RawCapturePolicies[0].Enabled {
		t.Fatalf("raw capture policies = %#v, want one enabled policy", cfg.RawCapturePolicies)
	}
	if len(cfg.Clients) != 1 || cfg.Clients[0].ID != "client-id" {
		t.Fatalf("clients = %#v, want client-id", cfg.Clients)
	}
	if len(cfg.RouteLimits) != 1 || len(cfg.ClientRouteLimitOverrides) != 1 {
		t.Fatalf("limits = (%d, %d), want (1, 1)", len(cfg.RouteLimits), len(cfg.ClientRouteLimitOverrides))
	}
	if cfg.VerdictProvider != nil {
		t.Fatal("global verdict provider = non-nil, want route-scoped provider only")
	}
	if cfg.VerdictProviders["guard"] == nil {
		t.Fatal("route-scoped verdict provider missing")
	}
}

func TestBuildConfigAppliesSingleGlobalVerdictProvider(t *testing.T) {
	cfg, err := BuildConfig(RuntimeConfig{
		Routes: []RouteConfig{{
			Key:            "chat",
			Method:         "POST",
			Path:           "/v1/chat/completions",
			ProviderKey:    "openai",
			ExecutionMode:  string(ExecutionInline),
			FallbackAction: string(ActionAllow),
		}},
		Providers: []ProviderConfig{{Key: "openai", BaseURL: "https://api.openai.test"}},
		VerdictProviders: []VerdictProviderConfig{{
			Key:      "guard",
			Name:     "guard",
			Endpoint: "http://guard.test/evaluate",
			Enabled:  true,
		}},
	}, CompileOptions{}, nil)
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}
	if cfg.VerdictProvider == nil {
		t.Fatal("global verdict provider = nil, want fallback provider")
	}
}

func TestBuildConfigRejectsInvalidLimits(t *testing.T) {
	tests := []struct {
		name  string
		input RuntimeConfig
	}{
		{
			name: "unknown route limit route",
			input: RuntimeConfig{
				Routes:      []RouteConfig{{Key: "chat"}},
				RouteLimits: []RouteLimitConfig{{RouteKey: "missing", RequestsPerWindow: 1, Window: time.Second, MaxConcurrentRequests: 1, MaxBodyBytes: 1}},
			},
		},
		{
			name: "invalid route limit values",
			input: RuntimeConfig{
				Routes:      []RouteConfig{{Key: "chat"}},
				RouteLimits: []RouteLimitConfig{{RouteKey: "chat", RequestsPerWindow: 0, Window: time.Second, MaxConcurrentRequests: 1, MaxBodyBytes: 1}},
			},
		},
		{
			name: "unknown override client",
			input: RuntimeConfig{
				Routes:                    []RouteConfig{{Key: "chat"}},
				RouteLimits:               []RouteLimitConfig{{RouteKey: "chat", RequestsPerWindow: 1, Window: time.Second, MaxConcurrentRequests: 1, MaxBodyBytes: 1}},
				ClientRouteLimitOverrides: []ClientRouteLimitOverrideConfig{{ClientID: "missing", RouteKey: "chat", RequestsPerWindow: 1, Window: time.Second, MaxConcurrentRequests: 1, MaxBodyBytes: 1}},
			},
		},
		{
			name: "override without route limit",
			input: RuntimeConfig{
				Routes:                    []RouteConfig{{Key: "chat"}},
				GatewayClients:            []GatewayClientConfig{{ID: "client-id"}},
				ClientRouteLimitOverrides: []ClientRouteLimitOverrideConfig{{ClientID: "client-id", RouteKey: "chat", RequestsPerWindow: 1, Window: time.Second, MaxConcurrentRequests: 1, MaxBodyBytes: 1}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildConfig(tt.input, CompileOptions{}, nil)
			if err == nil {
				t.Fatal("BuildConfig() error = nil, want error")
			}
		})
	}
}

func TestBuildConfigRejectsAmbiguousRuntimeConfig(t *testing.T) {
	tests := []struct {
		name  string
		input RuntimeConfig
	}{
		{
			name: "duplicate route model mappings",
			input: RuntimeConfig{
				Routes: []RouteConfig{{Key: "chat"}},
				ModelMappings: []ModelMappingConfig{
					{Key: "first", RouteKey: "chat"},
					{Key: "second", RouteKey: "chat"},
				},
			},
		},
		{
			name: "multiple global verdict providers",
			input: RuntimeConfig{
				VerdictProviders: []VerdictProviderConfig{
					{Key: "guard-a", Name: "guard-a", Endpoint: "http://guard-a.test", Enabled: true},
					{Key: "guard-b", Name: "guard-b", Endpoint: "http://guard-b.test", Enabled: true},
				},
			},
		},
		{
			name: "unknown route verdict provider",
			input: RuntimeConfig{
				VerdictProviders: []VerdictProviderConfig{{Key: "guard", Name: "guard", Endpoint: "http://guard.test", Enabled: true}},
				RouteVerdictProviderBindings: []RouteVerdictProviderBindingConfig{{
					RouteKey:           "chat",
					VerdictProviderKey: "missing",
				}},
			},
		},
		{
			name: "missing verdict provider key",
			input: RuntimeConfig{
				VerdictProviders: []VerdictProviderConfig{{Name: "", Endpoint: "http://guard.test", Enabled: true}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildConfig(tt.input, CompileOptions{}, nil)
			if err == nil {
				t.Fatal("BuildConfig() error = nil, want error")
			}
		})
	}
}

func TestBuildConfigRejectsMissingCredentials(t *testing.T) {
	input := RuntimeConfig{
		Providers: []ProviderConfig{{
			Key:           "openai",
			CredentialRef: "missing",
		}},
	}

	_, err := BuildConfig(input, CompileOptions{}, nil)
	if !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("BuildConfig() error = %v, want ErrCredentialNotFound", err)
	}
}

func TestBuildConfigResolvesVerdictProviderCredentials(t *testing.T) {
	input := RuntimeConfig{
		VerdictProviders: []VerdictProviderConfig{{
			Key:           "guard",
			Name:          "guard",
			Endpoint:      "http://guard.test",
			CredentialRef: "guard-key",
			Enabled:       true,
		}},
	}
	resolver := CredentialResolverFunc(func(ref string) (string, error) {
		if ref != "guard-key" {
			return "", ErrCredentialNotFound
		}
		return "guard-secret", nil
	})

	cfg, err := BuildConfig(input, CompileOptions{CredentialResolver: resolver}, nil)
	if err != nil {
		t.Fatalf("BuildConfig() error = %v", err)
	}
	if cfg.VerdictProvider == nil {
		t.Fatal("global verdict provider = nil, want credential-backed verdict provider")
	}
}

func TestBuildConfigRejectsMissingVerdictProviderCredentials(t *testing.T) {
	input := RuntimeConfig{
		VerdictProviders: []VerdictProviderConfig{{
			Key:           "guard",
			Name:          "guard",
			Endpoint:      "http://guard.test",
			CredentialRef: "missing",
			Enabled:       true,
		}},
	}

	_, err := BuildConfig(input, CompileOptions{}, nil)
	if !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("BuildConfig() error = %v, want ErrCredentialNotFound", err)
	}
}
