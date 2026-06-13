package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	configrevision "github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

func TestConvertPostgresRuntimeConfigMapsGatewayAndPolicyFields(t *testing.T) {
	raw := postgresRuntimeConfig{
		Revision: configrevision.ConfigRevision{Number: 7},
		Providers: []routingstore.Provider{{
			ID:      "provider-id",
			Name:    "openai",
			BaseURL: "https://upstream.example",
			Timeout: 2 * time.Second,
			Headers: json.RawMessage(`{"X-Test":"yes"}`),
		}},
		ModelMappings: []routingstore.ModelMapping{{
			ID:               "mapping-id",
			Name:             "chat-map",
			SourceModel:      "gpt-requested",
			TargetProviderID: "provider-id",
			TargetModel:      "gpt-upstream",
			Parameters:       json.RawMessage(`{"route_key":"chat","provider":"openai","enabled":false}`),
		}},
		Routes: []routingstore.Route{{
			ID:             "route-id",
			Name:           "chat",
			Enabled:        true,
			Method:         "POST",
			Path:           "/v1/chat/completions",
			Provider:       "openai",
			ModelMappingID: "mapping-id",
			ExecutionMode:  "inline",
			FallbackAction: "block",
		}},
		VerdictProviders: []routingstore.VerdictProvider{{
			Name:          "sec-model",
			Endpoint:      "https://verdict.example/evaluate",
			CredentialRef: "secret/verdict",
			Enabled:       true,
			Timeout:       1500 * time.Millisecond,
		}},
		Sinks: []observabilitystore.Sink{{
			ID:      "sink-id",
			Name:    "events",
			Kind:    "clickhouse",
			Enabled: true,
			Config:  json.RawMessage(`{"database":"kiwiguard"}`),
		}},
		Retention: []observabilitystore.RetentionPolicy{{
			ID:            "retention-id",
			Name:          "events-30d",
			SinkID:        "sink-id",
			EventType:     "*",
			RetentionDays: 30,
		}},
		PolicyBundles: []policystore.Bundle{{
			Key:           "pii",
			Version:       "1",
			Source:        string(policy.SourceUser),
			DefaultAction: string(policy.ActionAllow),
			Enabled:       true,
			Detectors: []policystore.Detector{{
				Key:        "email",
				Kind:       string(detection.KindRegex),
				Pattern:    `[a-z]+@[a-z]+\.com`,
				Categories: []string{"pii.email"},
				Enabled:    true,
			}},
			Rules: []policystore.Rule{{
				Key:          "block-email",
				Enabled:      true,
				Severity:     string(policy.SeverityHigh),
				Action:       string(policy.ActionBlock),
				DetectorKeys: []string{"email"},
				Scopes: []policystore.RuleScope{{
					Model:     "gpt-upstream",
					Direction: string(detection.DirectionInput),
				}},
			}},
		}},
		RawCapture: []observabilitystore.RawCapturePolicy{{
			ID:            "capture-id",
			RouteID:       "route-id",
			Direction:     "both",
			Enabled:       true,
			SampleRate:    1,
			RedactionMode: "none",
		}},
	}

	converted, err := convertPostgresRuntimeConfig(raw)
	if err != nil {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v", err)
	}

	if converted.Revision.Number != 7 {
		t.Fatalf("revision number = %d, want 7", converted.Revision.Number)
	}
	if len(converted.Routes) != 1 {
		t.Fatalf("routes len = %d, want 1", len(converted.Routes))
	}
	route := converted.Routes[0]
	if route.ProviderKey != "openai" {
		t.Fatalf("ProviderKey = %q, want openai", route.ProviderKey)
	}
	if route.RequestedModel != "gpt-requested" || route.UpstreamModel != "gpt-upstream" {
		t.Fatalf("route model mapping = %+v, want requested gpt-requested upstream gpt-upstream", route)
	}
	if len(converted.Providers) != 1 || converted.Providers[0].Headers["X-Test"] != "yes" {
		t.Fatalf("providers = %+v, want header mapping", converted.Providers)
	}
	if len(converted.PolicyBundles) != 1 || converted.PolicyBundles[0].Rules[0].Scope.Direction != detection.DirectionInput {
		t.Fatalf("policy bundles = %+v, want input-scoped rule", converted.PolicyBundles)
	}
	if len(converted.ModelMappings) != 1 {
		t.Fatalf("model mappings len = %d, want 1", len(converted.ModelMappings))
	}
	modelMapping := converted.ModelMappings[0]
	if modelMapping.RouteKey != "chat" || modelMapping.ProviderKey != "openai" || !modelMapping.Disabled {
		t.Fatalf("model mapping = %+v, want decoded route/provider/disabled parameters", modelMapping)
	}
	if len(converted.RawCapture) != 1 {
		t.Fatalf("raw capture len = %d, want 1", len(converted.RawCapture))
	}
	if len(converted.VerdictProviders) != 1 || converted.VerdictProviders[0].CredentialRef != "secret/verdict" {
		t.Fatalf("verdict providers = %+v, want credential ref preserved", converted.VerdictProviders)
	}
	capture := converted.RawCapture[0]
	if capture.ID != "capture-id" || capture.RouteKey != "chat" || capture.Direction != "both" || !capture.Enabled || capture.SampleRate != 1 || capture.RedactionMode != "none" {
		t.Fatalf("raw capture = %+v, want mapped capture policy", capture)
	}
	if len(converted.Sinks) != 1 {
		t.Fatalf("sinks len = %d, want 1", len(converted.Sinks))
	}
	sink := converted.Sinks[0]
	if sink.ID != "sink-id" || sink.Key != "events" || sink.Kind != "clickhouse" || sink.Disabled {
		t.Fatalf("sink = %+v, want enabled clickhouse sink", sink)
	}
	if len(converted.Retention) != 1 {
		t.Fatalf("retention len = %d, want 1", len(converted.Retention))
	}
	retention := converted.Retention[0]
	if retention.ID != "retention-id" || retention.Key != "events-30d" || retention.SinkKey != "events" || retention.RetentionDays != 30 {
		t.Fatalf("retention = %+v, want mapped 30 day retention policy", retention)
	}
}

func TestPostgresRuntimeRepositoryConvertsGatewayClientsAndLimits(t *testing.T) {
	raw := postgresRuntimeConfig{
		Revision: configrevision.ConfigRevision{Number: 9},
		Routes: []routingstore.Route{{
			ID:             "route-id",
			Name:           "chat",
			Enabled:        true,
			Method:         "POST",
			Path:           "/v1/chat/completions",
			Provider:       "openai",
			ExecutionMode:  "inline",
			FallbackAction: "block",
		}},
		GatewayClients: []clientstore.GatewayClient{{
			ID:         "postgres-client-id",
			ExternalID: "client-a",
			Name:       "Client A",
			Status:     "enabled",
			KeyPrefix:  "kgc_client-a",
			KeyHash:    "sha256:client-a",
		}},
		RouteLimitPolicies: []limitstore.RoutePolicy{{
			RouteID:               "route-id",
			RequestsPerWindow:     120,
			WindowSeconds:         60,
			MaxConcurrentRequests: 8,
			MaxBodyBytes:          1_048_576,
			Enabled:               true,
		}},
		ClientRouteLimitOverrides: []limitstore.ClientRouteOverride{{
			ClientID:              "postgres-client-id",
			RouteID:               "route-id",
			RequestsPerWindow:     40,
			WindowSeconds:         30,
			MaxConcurrentRequests: 3,
			MaxBodyBytes:          262_144,
			Enabled:               true,
		}},
	}

	converted, err := convertPostgresRuntimeConfig(raw)
	if err != nil {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v", err)
	}

	if len(converted.GatewayClients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(converted.GatewayClients))
	}
	client := converted.GatewayClients[0]
	if client.ID != "client-a" || client.Name != "Client A" || client.Status != "enabled" || client.KeyPrefix != "kgc_client-a" || client.KeyHash != "sha256:client-a" {
		t.Fatalf("GatewayClients[0] = %+v, want external client-a", client)
	}
	if len(converted.RouteLimits) != 1 {
		t.Fatalf("RouteLimits len = %d, want 1", len(converted.RouteLimits))
	}
	routeLimit := converted.RouteLimits[0]
	if routeLimit.RouteKey != "chat" || routeLimit.RequestsPerWindow != 120 || routeLimit.Window != time.Minute || routeLimit.MaxConcurrentRequests != 8 || routeLimit.MaxBodyBytes != 1_048_576 || routeLimit.Disabled {
		t.Fatalf("RouteLimits[0] = %+v, want chat default limit", routeLimit)
	}
	if len(converted.ClientRouteLimitOverrides) != 1 {
		t.Fatalf("ClientRouteLimitOverrides len = %d, want 1", len(converted.ClientRouteLimitOverrides))
	}
	override := converted.ClientRouteLimitOverrides[0]
	if override.ClientID != "client-a" || override.RouteKey != "chat" || override.RequestsPerWindow != 40 || override.Window != 30*time.Second || override.MaxConcurrentRequests != 3 || override.MaxBodyBytes != 262_144 || override.Disabled {
		t.Fatalf("ClientRouteLimitOverrides[0] = %+v, want client-a chat override", override)
	}
}

func TestConvertPostgresRuntimeConfigAppliesFallbacksAndSkipsDisabledBundles(t *testing.T) {
	enabled := true
	raw := postgresRuntimeConfig{
		Revision: configrevision.ConfigRevision{Number: 8},
		Providers: []routingstore.Provider{{
			ID:      "provider-id",
			Name:    "openai",
			BaseURL: "https://upstream.example",
		}},
		ModelMappings: []routingstore.ModelMapping{{
			ID:               "mapping-id",
			Name:             "fallback-map",
			SourceModel:      "gpt-requested",
			TargetProviderID: "provider-id",
			TargetModel:      "gpt-upstream",
			Parameters:       mustMarshalJSON(t, postgresModelMappingParams{Enabled: &enabled}),
		}},
		Routes: []routingstore.Route{{
			ID:             "route-id",
			Name:           "chat-prefix",
			Enabled:        false,
			Method:         "POST",
			PathPrefix:     "/v1/chat",
			Provider:       "openai",
			ModelMappingID: "mapping-id",
			ExecutionMode:  "shadow",
		}},
		PolicyBundles: []policystore.Bundle{
			{Key: "disabled", Version: "1", Source: string(policy.SourceUser), Enabled: false},
			{Key: "enabled", Version: "1", Source: string(policy.SourceUser), Enabled: true},
		},
	}

	converted, err := convertPostgresRuntimeConfig(raw)
	if err != nil {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v", err)
	}

	if len(converted.Routes) != 1 {
		t.Fatalf("routes len = %d, want 1", len(converted.Routes))
	}
	route := converted.Routes[0]
	if route.Path != "/v1/chat" || !route.Disabled {
		t.Fatalf("route = %+v, want path prefix fallback and disabled route", route)
	}
	if len(converted.ModelMappings) != 1 {
		t.Fatalf("model mappings len = %d, want 1", len(converted.ModelMappings))
	}
	mapping := converted.ModelMappings[0]
	if mapping.ProviderKey != "openai" || mapping.RouteKey != "" || mapping.Disabled {
		t.Fatalf("model mapping = %+v, want provider-name fallback and enabled mapping", mapping)
	}
	if len(converted.PolicyBundles) != 1 || converted.PolicyBundles[0].Key != "enabled" {
		t.Fatalf("policy bundles = %+v, want only enabled bundle", converted.PolicyBundles)
	}
}

func TestPostgresRuntimeRepositoryLoadsConvertedConfig(t *testing.T) {
	repo := NewRepository(fakePostgresRuntimeRepository{
		activeRevision: 7,
		config: postgresRuntimeConfig{
			Revision: configrevision.ConfigRevision{Number: 7},
			Providers: []routingstore.Provider{{
				ID:      "provider-id",
				Name:    "openai",
				BaseURL: "https://upstream.example",
			}},
			Routes: []routingstore.Route{{
				Name:          "chat",
				Enabled:       true,
				Method:        "POST",
				Path:          "/v1/chat/completions",
				Provider:      "openai",
				ExecutionMode: "inline",
			}},
			PolicyBundles: []policystore.Bundle{{
				Key:           "empty",
				Version:       "1",
				Source:        string(policy.SourceUser),
				DefaultAction: string(policy.ActionAllow),
				Enabled:       true,
			}},
		},
	})

	revision, err := repo.ActiveRevisionNumber(t.Context())
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() error = %v", err)
	}
	if revision != 7 {
		t.Fatalf("ActiveRevisionNumber() = %d, want 7", revision)
	}

	cfg, err := repo.LoadRuntimeConfig(t.Context())
	if err != nil {
		t.Fatalf("LoadRuntimeConfig() error = %v", err)
	}
	if cfg.Revision.Number != 7 || len(cfg.Routes) != 1 || len(cfg.PolicyBundles) != 1 {
		t.Fatalf("runtime config = %+v, want revision/routes/bundles", cfg)
	}
}

func mustMarshalJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

func TestPostgresRuntimeRepositoryMapsNotFound(t *testing.T) {
	repo := NewRepository(fakePostgresRuntimeRepository{err: configstore.ErrActiveConfigNotFound})

	_, err := repo.ActiveRevisionNumber(t.Context())
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("ActiveRevisionNumber() error = %v, want ErrActiveRuntimeConfigNotFound", err)
	}
	_, err = repo.LoadRuntimeConfig(t.Context())
	if !errors.Is(err, ErrActiveRuntimeConfigNotFound) {
		t.Fatalf("LoadRuntimeConfig() error = %v, want ErrActiveRuntimeConfigNotFound", err)
	}
}

func TestPostgresRuntimeRepositoryPreservesGenericErrors(t *testing.T) {
	wantErr := errors.New("postgres unavailable")
	repo := NewRepository(fakePostgresRuntimeRepository{err: wantErr})

	if _, err := repo.ActiveRevisionNumber(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("ActiveRevisionNumber() error = %v, want %v", err, wantErr)
	}
	if _, err := repo.LoadRuntimeConfig(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("LoadRuntimeConfig() error = %v, want %v", err, wantErr)
	}
}

func TestPostgresRuntimeRepositoryRequiresRepository(t *testing.T) {
	repo := NewRepository(nil)

	if _, err := repo.ActiveRevisionNumber(t.Context()); err == nil {
		t.Fatal("ActiveRevisionNumber() error = nil, want repository required error")
	}
	if _, err := repo.LoadRuntimeConfig(t.Context()); err == nil {
		t.Fatal("LoadRuntimeConfig() error = nil, want repository required error")
	}
}

func TestConvertPostgresRuntimeConfigHandlesEmptyOptionalJSON(t *testing.T) {
	converted, err := convertPostgresRuntimeConfig(postgresRuntimeConfig{
		Revision: configrevision.ConfigRevision{Number: 1},
		Providers: []routingstore.Provider{{
			ID:      "provider-id",
			Name:    "openai",
			BaseURL: "https://upstream.example",
		}},
		ModelMappings: []routingstore.ModelMapping{{
			ID:               "mapping-id",
			Name:             "default",
			SourceModel:      "gpt-requested",
			TargetProviderID: "provider-id",
			TargetModel:      "gpt-upstream",
		}},
	})
	if err != nil {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v", err)
	}
	if len(converted.Providers) != 1 || converted.Providers[0].Headers != nil {
		t.Fatalf("providers = %+v, want nil optional headers", converted.Providers)
	}
	if len(converted.ModelMappings) != 1 {
		t.Fatalf("model mappings len = %d, want 1", len(converted.ModelMappings))
	}
	mapping := converted.ModelMappings[0]
	if mapping.ProviderKey != "openai" || mapping.RouteKey != "" || mapping.Disabled {
		t.Fatalf("model mapping = %+v, want provider fallback with enabled default", mapping)
	}
}

func TestConvertPostgresRuntimeConfigRejectsInvalidProviderHeaders(t *testing.T) {
	_, err := convertPostgresRuntimeConfig(postgresRuntimeConfig{
		Revision:  configrevision.ConfigRevision{Number: 1},
		Providers: []routingstore.Provider{{Name: "bad", Headers: json.RawMessage(`{"X":1}`)}},
	})
	if err == nil || !strings.Contains(err.Error(), "decode provider bad headers") {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v, want provider headers error", err)
	}
}

func TestPostgresRuntimeRepositoryWrapsConversionError(t *testing.T) {
	repo := NewRepository(fakePostgresRuntimeRepository{
		config: postgresRuntimeConfig{
			Revision:  configrevision.ConfigRevision{Number: 1},
			Providers: []routingstore.Provider{{Name: "bad", Headers: json.RawMessage(`{"X":1}`)}},
		},
	})

	_, err := repo.LoadRuntimeConfig(t.Context())
	if err == nil {
		t.Fatal("LoadRuntimeConfig() error = nil, want conversion error")
	}
	if !strings.Contains(err.Error(), "convert postgres runtime config") {
		t.Fatalf("LoadRuntimeConfig() error = %v, want conversion context", err)
	}
}

func TestConvertPostgresRuntimeConfigRejectsInvalidModelMappingParameters(t *testing.T) {
	_, err := convertPostgresRuntimeConfig(postgresRuntimeConfig{
		Revision: configrevision.ConfigRevision{Number: 1},
		ModelMappings: []routingstore.ModelMapping{{
			Name:       "bad-map",
			Parameters: json.RawMessage(`{"enabled":"yes"}`),
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "decode model mapping bad-map parameters") {
		t.Fatalf("convertPostgresRuntimeConfig() error = %v, want model mapping parameters error", err)
	}
}

func TestConvertPostgresPolicyBundleSkipsInactiveChildrenAndSplitsScopes(t *testing.T) {
	bundle := convertPostgresPolicyBundle(policystore.Bundle{
		Key:           "pii",
		Version:       "1",
		Source:        string(policy.SourceUser),
		DefaultAction: "",
		Detectors: []policystore.Detector{
			{Key: "disabled", Kind: string(detection.KindEmail), Enabled: false},
			{Key: "email", Kind: string(detection.KindEmail), Enabled: true},
		},
		Rules: []policystore.Rule{
			{Key: "disabled-rule", Enabled: false, DetectorKeys: []string{"email"}},
			{
				Key:          "scoped",
				Enabled:      true,
				Severity:     string(policy.SeverityHigh),
				Action:       string(policy.ActionBlock),
				DetectorKeys: []string{"email"},
				Scopes: []policystore.RuleScope{
					{RouteID: "route-id", ProviderID: "provider-id", Model: "gpt-a", Direction: string(detection.DirectionInput)},
					{RouteID: "route-id", ProviderID: "provider-id", Model: "gpt-b", Direction: string(detection.DirectionOutput)},
				},
			},
			{Key: "unscoped", Enabled: true, Severity: string(policy.SeverityLow), Action: string(policy.ActionShadowLog), DetectorKeys: []string{"email"}},
		},
	}, map[string]string{"route-id": "chat"}, map[string]string{"provider-id": "openai"})

	if bundle.DefaultAction != policy.ActionAllow {
		t.Fatalf("DefaultAction = %q, want allow fallback", bundle.DefaultAction)
	}
	if len(bundle.Detectors) != 1 || bundle.Detectors[0].Key != "email" {
		t.Fatalf("Detectors = %+v, want only enabled email detector", bundle.Detectors)
	}
	if len(bundle.Rules) != 3 {
		t.Fatalf("Rules len = %d, want two scoped rules plus one unscoped", len(bundle.Rules))
	}
	if bundle.Rules[0].Key != "scoped" || bundle.Rules[1].Key != "scoped-scope-2" {
		t.Fatalf("scoped rule keys = %q/%q, want scoped split keys", bundle.Rules[0].Key, bundle.Rules[1].Key)
	}
	if bundle.Rules[0].Scope.RouteKey != "chat" || bundle.Rules[1].Scope.Direction != detection.DirectionOutput {
		t.Fatalf("scoped rules = %+v, want mapped route/provider and directions", bundle.Rules[:2])
	}
	if bundle.Rules[2].Scope != (policy.Scope{}) {
		t.Fatalf("unscoped rule scope = %+v, want empty scope", bundle.Rules[2].Scope)
	}
}

type fakePostgresRuntimeRepository struct {
	activeRevision int64
	config         postgresRuntimeConfig
	err            error
}

func (r fakePostgresRuntimeRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.activeRevision, nil
}

func (r fakePostgresRuntimeRepository) LoadRuntimeConfig(ctx context.Context) (postgresRuntimeConfig, error) {
	if r.err != nil {
		return postgresRuntimeConfig{}, r.err
	}
	return r.config, nil
}
