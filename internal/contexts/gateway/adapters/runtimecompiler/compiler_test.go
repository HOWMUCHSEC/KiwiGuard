package runtimecompiler

import (
	"strings"
	"testing"
	"time"

	gatewayhttp "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters/http/openai"
	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestCompileGatewayRuntimeBuildsRoutesProvidersAndSnapshot(t *testing.T) {
	writer := events.NewMemoryWriter(events.MemoryWriterOptions{Capacity: 4})
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 42},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
			APIKey:  "secret",
		}},
		Routes: []RouteConfig{{
			Key:            "chat",
			Method:         "POST",
			Path:           "/v1/chat/completions",
			ProviderKey:    "openai",
			RequestedModel: "gpt-test",
			MappedModel:    "gpt-test",
			UpstreamModel:  "gpt-upstream",
			ExecutionMode:  "inline",
			FallbackAction: "block",
		}},
		VerdictProviders: []VerdictProviderConfig{{
			Key:      "sec",
			Name:     "security",
			Endpoint: "https://verdict.example/evaluate",
			Enabled:  true,
		}},
		RawCapture: []RawCaptureConfig{{
			ID:            "capture-id",
			RouteKey:      "chat",
			Direction:     "both",
			Enabled:       true,
			SampleRate:    1,
			RedactionMode: "none",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1.0.0",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{
		MaxBodyBytes:    4096,
		UpstreamTimeout: 750 * time.Millisecond,
		VerdictTimeout:  1500 * time.Millisecond,
		EventWriter:     writer,
	})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}

	if compiled.RevisionNumber != 42 {
		t.Fatalf("RevisionNumber = %d, want 42", compiled.RevisionNumber)
	}
	if compiled.SnapshotHash == "" {
		t.Fatal("SnapshotHash is empty")
	}
	if len(compiledGatewayConfig(t, compiled).Routes) != 1 {
		t.Fatalf("Routes len = %d, want 1", len(compiledGatewayConfig(t, compiled).Routes))
	}
	route := compiledGatewayConfig(t, compiled).Routes[0]
	if route.Path != "/v1/chat/completions" {
		t.Fatalf("route path = %q, want /v1/chat/completions", route.Path)
	}
	if route.ModelMapping.Requested != "gpt-test" || route.ModelMapping.Upstream != "gpt-upstream" {
		t.Fatalf("model mapping = %+v, want requested gpt-test upstream gpt-upstream", route.ModelMapping)
	}
	if len(compiledGatewayConfig(t, compiled).Providers) != 1 || compiledGatewayConfig(t, compiled).Providers[0].BaseURL != "https://upstream.example" {
		t.Fatalf("Providers = %+v, want upstream provider", compiledGatewayConfig(t, compiled).Providers)
	}
	if compiledGatewayConfig(t, compiled).EventWriter != writer {
		t.Fatal("EventWriter was not wired")
	}
	if compiledGatewayConfig(t, compiled).UpstreamTimeout != 750*time.Millisecond {
		t.Fatalf("UpstreamTimeout = %v, want 750ms", compiledGatewayConfig(t, compiled).UpstreamTimeout)
	}
	if compiledGatewayConfig(t, compiled).VerdictTimeout != 1500*time.Millisecond {
		t.Fatalf("VerdictTimeout = %v, want 1.5s", compiledGatewayConfig(t, compiled).VerdictTimeout)
	}
	if compiledGatewayConfig(t, compiled).VerdictProvider == nil {
		t.Fatal("VerdictProvider is nil")
	}
	if len(compiledGatewayConfig(t, compiled).RawCapturePolicies) != 1 {
		t.Fatalf("RawCapturePolicies len = %d, want 1", len(compiledGatewayConfig(t, compiled).RawCapturePolicies))
	}
	capture := compiledGatewayConfig(t, compiled).RawCapturePolicies[0]
	if capture.ID != "capture-id" || capture.RouteKey != "chat" || capture.Direction != "both" || !capture.Enabled {
		t.Fatalf("RawCapturePolicies[0] = %+v, want compiled capture policy", capture)
	}
}

func TestCompileGatewayRuntimeIncludesGatewayClientsAndLimits(t *testing.T) {
	input := RuntimeConfig{
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
		GatewayClients: []GatewayClientConfig{{
			ID:        "client-a",
			Name:      "Client A",
			Status:    "enabled",
			KeyPrefix: "kgc_client-a",
			KeyHash:   "sha256:client-a",
		}},
		RouteLimits: []RouteLimitConfig{{
			RouteKey:              "chat",
			RequestsPerWindow:     120,
			Window:                time.Minute,
			MaxConcurrentRequests: 8,
			MaxBodyBytes:          1_048_576,
		}},
		ClientRouteLimitOverrides: []ClientRouteLimitOverrideConfig{{
			ClientID:              "client-a",
			RouteKey:              "chat",
			RequestsPerWindow:     40,
			Window:                30 * time.Second,
			MaxConcurrentRequests: 3,
			MaxBodyBytes:          262_144,
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}

	if len(compiledGatewayConfig(t, compiled).Clients) != 1 {
		t.Fatalf("Clients len = %d, want 1", len(compiledGatewayConfig(t, compiled).Clients))
	}
	client := compiledGatewayConfig(t, compiled).Clients[0]
	if client.ID != "client-a" || client.Name != "Client A" || string(client.Status) != "enabled" || client.KeyPrefix != "kgc_client-a" || client.KeyHash != "sha256:client-a" {
		t.Fatalf("Clients[0] = %+v, want compiled client-a", client)
	}
	if len(compiledGatewayConfig(t, compiled).RouteLimits) != 1 {
		t.Fatalf("RouteLimits len = %d, want 1", len(compiledGatewayConfig(t, compiled).RouteLimits))
	}
	routeLimit := compiledGatewayConfig(t, compiled).RouteLimits[0]
	if routeLimit.RouteKey != "chat" || routeLimit.RequestsPerWindow != 120 || routeLimit.Window != time.Minute || routeLimit.MaxConcurrentRequests != 8 || routeLimit.MaxBodyBytes != 1_048_576 || !routeLimit.Enabled {
		t.Fatalf("RouteLimits[0] = %+v, want chat default limit", routeLimit)
	}
	if len(compiledGatewayConfig(t, compiled).ClientRouteLimitOverrides) != 1 {
		t.Fatalf("ClientRouteLimitOverrides len = %d, want 1", len(compiledGatewayConfig(t, compiled).ClientRouteLimitOverrides))
	}
	override := compiledGatewayConfig(t, compiled).ClientRouteLimitOverrides[0]
	if override.ClientID != "client-a" || override.RouteKey != "chat" || override.RequestsPerWindow != 40 || override.Window != 30*time.Second || override.MaxConcurrentRequests != 3 || override.MaxBodyBytes != 262_144 || !override.Enabled {
		t.Fatalf("ClientRouteLimitOverrides[0] = %+v, want client-a chat override", override)
	}
	if len(compiledGatewayConfig(t, compiled).Routes) != 1 || !compiledGatewayConfig(t, compiled).Routes[0].RequireClientAuth {
		t.Fatalf("Routes = %+v, want chat route requiring client auth", compiledGatewayConfig(t, compiled).Routes)
	}
}

func TestCompileGatewayRuntimeProtectsRoutesWhenGatewayClientsExistWithoutRouteLimits(t *testing.T) {
	input := RuntimeConfig{
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
		GatewayClients: []GatewayClientConfig{{
			ID:        "client-a",
			Name:      "Client A",
			Status:    "enabled",
			KeyPrefix: "kgc_client-a",
			KeyHash:   "sha256:client-a",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}

	if len(compiledGatewayConfig(t, compiled).Routes) != 1 || !compiledGatewayConfig(t, compiled).Routes[0].RequireClientAuth {
		t.Fatalf("Routes = %+v, want route requiring client auth even without route limits", compiledGatewayConfig(t, compiled).Routes)
	}
}

func TestCompileGatewayRuntimeRejectsLimitForUnknownRoute(t *testing.T) {
	base := RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        "POST",
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		GatewayClients: []GatewayClientConfig{{
			ID:        "client-a",
			Status:    "enabled",
			KeyPrefix: "kgc_client-a",
			KeyHash:   "sha256:client-a",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	tests := []struct {
		name    string
		mutate  func(*RuntimeConfig)
		wantErr string
	}{
		{
			name: "route limit unknown route",
			mutate: func(input *RuntimeConfig) {
				input.RouteLimits = []RouteLimitConfig{{RouteKey: "missing", RequestsPerWindow: 1, Window: time.Minute, MaxConcurrentRequests: 1, MaxBodyBytes: 1024}}
			},
			wantErr: `route limit references unknown route "missing"`,
		},
		{
			name: "override unknown route",
			mutate: func(input *RuntimeConfig) {
				input.ClientRouteLimitOverrides = []ClientRouteLimitOverrideConfig{{ClientID: "client-a", RouteKey: "missing", RequestsPerWindow: 1, Window: time.Minute, MaxConcurrentRequests: 1, MaxBodyBytes: 1024}}
			},
			wantErr: `client route limit override references unknown route "missing"`,
		},
		{
			name: "override unknown client",
			mutate: func(input *RuntimeConfig) {
				input.ClientRouteLimitOverrides = []ClientRouteLimitOverrideConfig{{ClientID: "missing", RouteKey: "chat", RequestsPerWindow: 1, Window: time.Minute, MaxConcurrentRequests: 1, MaxBodyBytes: 1024}}
			},
			wantErr: `client route limit override references unknown client "missing"`,
		},
		{
			name: "override missing route limit",
			mutate: func(input *RuntimeConfig) {
				input.ClientRouteLimitOverrides = []ClientRouteLimitOverrideConfig{{ClientID: "client-a", RouteKey: "chat", RequestsPerWindow: 1, Window: time.Minute, MaxConcurrentRequests: 1, MaxBodyBytes: 1024}}
			},
			wantErr: `client route limit override references route "chat" without an enabled route limit`,
		},
		{
			name: "route limit invalid values",
			mutate: func(input *RuntimeConfig) {
				input.RouteLimits = []RouteLimitConfig{{RouteKey: "chat", RequestsPerWindow: 0, Window: time.Minute, MaxConcurrentRequests: 1, MaxBodyBytes: 1024}}
			},
			wantErr: "limit values must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := base
			tt.mutate(&input)

			_, err := CompileGatewayRuntime(input, CompileOptions{})
			if err == nil {
				t.Fatal("CompileGatewayRuntime() error = nil, want limit validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("CompileGatewayRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestCompileGatewayRuntimeRejectsMissingRevision(t *testing.T) {
	_, err := CompileGatewayRuntime(RuntimeConfig{}, CompileOptions{})
	if err == nil {
		t.Fatal("CompileGatewayRuntime() error = nil, want revision error")
	}
}

func TestCompileGatewayRuntimeSkipsDisabledRecordsAndClonesHeaders(t *testing.T) {
	headers := map[string]string{"X-Test": "yes"}
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{
			{Key: "disabled", Disabled: true},
			{Key: "openai", BaseURL: "https://upstream.example", Headers: headers},
			{Key: "anthropic", BaseURL: "https://anthropic.example"},
		},
		Routes: []RouteConfig{
			{Key: "disabled", Disabled: true},
			{Key: "chat", Method: "POST", Path: "/v1/chat/completions", ProviderKey: "openai", ExecutionMode: "inline"},
		},
		ModelMappings: []ModelMappingConfig{
			{Key: "disabled", RouteKey: "disabled", Disabled: true},
			{Key: "chat-map", RouteKey: "chat", ProviderKey: "anthropic", RequestedModel: "gpt-requested", UpstreamModel: "gpt-upstream"},
		},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}
	if len(compiledGatewayConfig(t, compiled).Routes) != 1 || compiledGatewayConfig(t, compiled).Routes[0].Key != "chat" {
		t.Fatalf("Routes = %+v, want only chat", compiledGatewayConfig(t, compiled).Routes)
	}
	if len(compiledGatewayConfig(t, compiled).Providers) != 2 {
		t.Fatalf("Providers = %+v, want openai and anthropic", compiledGatewayConfig(t, compiled).Providers)
	}
	headers["X-Test"] = "mutated"
	if compiledGatewayConfig(t, compiled).Providers[0].Headers["X-Test"] != "yes" {
		t.Fatalf("provider headers were not cloned: %+v", compiledGatewayConfig(t, compiled).Providers[0].Headers)
	}
	if compiledGatewayConfig(t, compiled).Routes[0].ModelMapping.Requested != "gpt-requested" {
		t.Fatalf("ModelMapping = %+v, want route mapping override", compiledGatewayConfig(t, compiled).Routes[0].ModelMapping)
	}
	if compiledGatewayConfig(t, compiled).Routes[0].ProviderKey != "anthropic" {
		t.Fatalf("ProviderKey = %q, want anthropic from model mapping override", compiledGatewayConfig(t, compiled).Routes[0].ProviderKey)
	}
	if compiledGatewayConfig(t, compiled).VerdictProvider != nil {
		t.Fatal("VerdictProvider is non-nil with no enabled providers")
	}
}

func TestCompileGatewayRuntimeBindsVerdictProvidersByRoute(t *testing.T) {
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		Routes: []RouteConfig{
			{Key: "chat", Method: "POST", Path: "/v1/chat/completions", ProviderKey: "openai", ExecutionMode: "inline"},
			{Key: "responses", Method: "POST", Path: "/v1/responses", ProviderKey: "openai", ExecutionMode: "inline"},
		},
		VerdictProviders: []VerdictProviderConfig{
			{Key: "sec-chat", Name: "chat-security", Endpoint: "https://chat-security.example/evaluate", Enabled: true},
			{Key: "sec-responses", Name: "responses-security", Endpoint: "https://responses-security.example/evaluate", Enabled: true},
		},
		RouteVerdictProviderBindings: []RouteVerdictProviderBindingConfig{
			{RouteKey: "responses", VerdictProviderKey: "sec-responses", ExecutionMode: "async_shadow", Priority: 20},
			{RouteKey: "chat", VerdictProviderKey: "sec-chat", ExecutionMode: "inline", Priority: 10},
		},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}
	if len(compiledGatewayConfig(t, compiled).VerdictProviders) != 2 {
		t.Fatalf("VerdictProviders len = %d, want 2", len(compiledGatewayConfig(t, compiled).VerdictProviders))
	}
	routes := map[string]gatewayRouteVerdict{}
	for _, route := range compiledGatewayConfig(t, compiled).Routes {
		routes[route.Key] = gatewayRouteVerdict{
			providerKey: route.VerdictProviderKey,
			execution:   string(route.Execution),
		}
	}
	if routes["chat"].providerKey != "sec-chat" || routes["chat"].execution != "inline" {
		t.Fatalf("chat route verdict binding = %+v, want sec-chat inline", routes["chat"])
	}
	if routes["responses"].providerKey != "sec-responses" || routes["responses"].execution != "async_shadow" {
		t.Fatalf("responses route verdict binding = %+v, want sec-responses async_shadow", routes["responses"])
	}
	if compiledGatewayConfig(t, compiled).VerdictProvider != nil {
		t.Fatal("global VerdictProvider is non-nil with route-specific bindings")
	}
}

func TestCompileGatewayRuntimeUsesGlobalVerdictProviderWhenBindingsAreInactive(t *testing.T) {
	input := RuntimeConfig{
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
			Key:      "sec",
			Name:     "security",
			Endpoint: "https://security.example/evaluate",
			Enabled:  true,
		}},
		RouteVerdictProviderBindings: []RouteVerdictProviderBindingConfig{{
			RouteKey:           "chat",
			VerdictProviderKey: "sec",
			ExecutionMode:      "inline",
			Disabled:           true,
			Priority:           10,
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	compiled, err := CompileGatewayRuntime(input, CompileOptions{})
	if err != nil {
		t.Fatalf("CompileGatewayRuntime() error = %v", err)
	}
	if compiledGatewayConfig(t, compiled).VerdictProvider == nil {
		t.Fatal("global VerdictProvider is nil, want fallback provider")
	}
	if compiledGatewayConfig(t, compiled).Routes[0].VerdictProviderKey != "" {
		t.Fatalf("route VerdictProviderKey = %q, want no route-specific binding", compiledGatewayConfig(t, compiled).Routes[0].VerdictProviderKey)
	}
}

func TestCompileGatewayRuntimeRejectsDuplicateActiveRouteModelMappings(t *testing.T) {
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		Routes: []RouteConfig{{
			Key:           "chat",
			Method:        "POST",
			Path:          "/v1/chat/completions",
			ProviderKey:   "openai",
			ExecutionMode: "inline",
		}},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		ModelMappings: []ModelMappingConfig{
			{Key: "first", RouteKey: "chat", RequestedModel: "gpt-a", UpstreamModel: "gpt-a-upstream"},
			{Key: "second", RouteKey: "chat", RequestedModel: "gpt-b", UpstreamModel: "gpt-b-upstream"},
		},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	_, err := CompileGatewayRuntime(input, CompileOptions{})
	if err == nil {
		t.Fatal("CompileGatewayRuntime() error = nil, want duplicate mapping error")
	}
	if !strings.Contains(err.Error(), "duplicate active model mappings for route") {
		t.Fatalf("CompileGatewayRuntime() error = %v, want duplicate mapping error", err)
	}
}

type gatewayRouteVerdict struct {
	providerKey string
	execution   string
}

func TestCompileGatewayRuntimeRejectsMultipleEnabledVerdictProviders(t *testing.T) {
	input := RuntimeConfig{
		Revision: RuntimeRevision{Number: 1},
		VerdictProviders: []VerdictProviderConfig{
			{Name: "first", Endpoint: "https://first.example/evaluate", Enabled: true},
			{Name: "second", Endpoint: "https://second.example/evaluate", Enabled: true},
		},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	}

	_, err := CompileGatewayRuntime(input, CompileOptions{})
	if err == nil {
		t.Fatal("CompileGatewayRuntime() error = nil, want multiple verdict providers error")
	}
	if !strings.Contains(err.Error(), "multiple enabled verdict providers") {
		t.Fatalf("CompileGatewayRuntime() error = %v, want multiple enabled verdict providers error", err)
	}
}

func TestGatewayRuntimeCompilerCompileRuntime(t *testing.T) {
	compiler := Compiler{}
	compiled, err := compiler.CompileRuntime(t.Context(), RuntimeConfig{
		Revision: RuntimeRevision{Number: 3},
		Routes:   []RouteConfig{{Key: "chat", Method: "POST", Path: "/v1/chat/completions", ProviderKey: "openai"}},
		Providers: []ProviderConfig{{
			Key:     "openai",
			BaseURL: "https://upstream.example",
		}},
		PolicyBundles: []policy.Bundle{{
			Key:           "empty",
			Version:       "1",
			Source:        policy.SourceBuiltIn,
			DefaultAction: policy.ActionAllow,
		}},
	})
	if err != nil {
		t.Fatalf("CompileRuntime() error = %v", err)
	}
	if compiled.RevisionNumber != 3 {
		t.Fatalf("RevisionNumber = %d, want 3", compiled.RevisionNumber)
	}
}

func compiledGatewayConfig(t *testing.T, compiled appruntime.CompiledRuntime) gatewayhttp.Config {
	t.Helper()
	config, err := gatewayhttp.GatewayConfigFromRuntime(compiled)
	if err != nil {
		t.Fatalf("GatewayConfigFromRuntime() error = %v", err)
	}
	return config
}
