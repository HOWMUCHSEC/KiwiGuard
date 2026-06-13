package configstore

import (
	"context"
	"errors"
	"testing"
	"time"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestConfigRepositoryLoadsActiveRuntimeConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if cfg.Revision.Number == 0 {
		t.Fatal("revision number was not loaded")
	}
	if len(cfg.Routes) != 1 || cfg.Routes[0].Name != "chat" {
		t.Fatalf("Routes = %+v, want one chat route", cfg.Routes)
	}
	if len(cfg.PolicyBundles) != 1 || cfg.PolicyBundles[0].Key != "pii" {
		t.Fatalf("PolicyBundles = %+v, want one pii bundle", cfg.PolicyBundles)
	}
	if len(cfg.Sinks) != 1 || cfg.Sinks[0].Kind != "clickhouse" {
		t.Fatalf("Sinks = %+v, want clickhouse sink", cfg.Sinks)
	}
	if len(cfg.Retention) != 1 || cfg.Retention[0].RetentionDays != 30 {
		t.Fatalf("Retention = %+v, want one 30 day policy", cfg.Retention)
	}
	if len(cfg.RawCapture) != 1 || cfg.RawCapture[0].RedactionMode != "redacted" {
		t.Fatalf("RawCapture = %+v, want one redacted policy", cfg.RawCapture)
	}
	if len(cfg.RouteVerdictProviderBindings) != 1 {
		t.Fatalf("RouteVerdictProviderBindings len = %d, want 1", len(cfg.RouteVerdictProviderBindings))
	}
	binding := cfg.RouteVerdictProviderBindings[0]
	if binding.RouteID == "" || binding.VerdictProviderID == "" || binding.ExecutionMode != "inline" || !binding.Enabled {
		t.Fatalf("RouteVerdictProviderBindings[0] = %+v, want enabled inline binding", binding)
	}
}

func TestConfigRepositoryCreatesDraftByCloningActiveRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	bundle := policystore.Bundle{
		Key:           "pii",
		Version:       "2026.06",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
		Detectors: []policystore.Detector{{
			Key:        "phone",
			Kind:       string(detection.KindRegex),
			Pattern:    "\\+?[0-9]{10,15}",
			Categories: []string{"pii.phone"},
			Enabled:    true,
		}},
		Rules: []policystore.Rule{{
			Key:          "block-phone",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"phone"},
			Scopes:       []policystore.RuleScope{{Direction: string(detection.DirectionInput)}},
		}},
	}
	if err := testUpsertPolicyBundle(ctx, repo, bundle); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}
	if _, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii"})); err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if len(cfg.Routes) != 1 || len(cfg.Providers) != 1 || len(cfg.ModelMappings) != 1 {
		t.Fatalf("runtime route/provider/mapping counts = %d/%d/%d, want 1/1/1", len(cfg.Routes), len(cfg.Providers), len(cfg.ModelMappings))
	}
	if len(cfg.VerdictProviders) != 1 || len(cfg.Sinks) != 1 || len(cfg.Retention) != 1 || len(cfg.RawCapture) != 1 {
		t.Fatalf("runtime verdict/sink/retention/raw counts = %d/%d/%d/%d, want 1/1/1/1", len(cfg.VerdictProviders), len(cfg.Sinks), len(cfg.Retention), len(cfg.RawCapture))
	}
	if len(cfg.PolicyBundles) != 1 || cfg.PolicyBundles[0].Version != "2026.06" {
		t.Fatalf("PolicyBundles = %+v, want updated pii bundle", cfg.PolicyBundles)
	}
	if cfg.Routes[0].ModelMappingID == "" || cfg.Retention[0].SinkID == "" || cfg.RawCapture[0].RouteID == "" {
		t.Fatalf("cloned dependency links were not preserved: route=%+v retention=%+v raw=%+v", cfg.Routes[0], cfg.Retention[0], cfg.RawCapture[0])
	}
}

func TestConfigRepositoryLoadsOptionalRelationshipsAsEmptyIDs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	_, err := pool.Exec(ctx, `
		update model_mappings set target_provider_id = null;
		update routes set model_mapping_id = null;
		update policy_rule_scopes set route_id = null, provider_id = null;
		update retention_policies set sink_id = null;
		update raw_capture_policies set route_id = null;
	`)
	if err != nil {
		t.Fatalf("clear optional relationships: %v", err)
	}

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if len(cfg.ModelMappings) != 1 || cfg.ModelMappings[0].TargetProviderID != "" {
		t.Fatalf("ModelMappings = %+v, want empty target provider ID", cfg.ModelMappings)
	}
	if len(cfg.Routes) != 1 || cfg.Routes[0].ModelMappingID != "" {
		t.Fatalf("Routes = %+v, want empty model mapping ID", cfg.Routes)
	}
	if len(cfg.PolicyBundles) != 1 || len(cfg.PolicyBundles[0].Rules) != 1 || len(cfg.PolicyBundles[0].Rules[0].Scopes) != 1 {
		t.Fatalf("PolicyBundles = %+v, want one scoped rule", cfg.PolicyBundles)
	}
	scope := cfg.PolicyBundles[0].Rules[0].Scopes[0]
	if scope.RouteID != "" || scope.ProviderID != "" {
		t.Fatalf("RuleScope = %+v, want empty route/provider IDs", scope)
	}
	if len(cfg.Retention) != 1 || cfg.Retention[0].SinkID != "" {
		t.Fatalf("Retention = %+v, want empty sink ID", cfg.Retention)
	}
	if len(cfg.RawCapture) != 1 || cfg.RawCapture[0].RouteID != "" {
		t.Fatalf("RawCapture = %+v, want empty route ID", cfg.RawCapture)
	}
}

func TestConfigRepositoryLoadActiveRuntimeConfigReturnsNotFoundWithoutActiveRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	_, err := testLoadActiveRuntimeConfig(ctx, repo)
	if !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v, want ErrActiveConfigNotFound", err)
	}
}

func TestActiveRevisionNumberReturnsNotFoundWithoutActiveRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	_, err := repo.ActiveRevisionNumber(ctx)
	if !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("ActiveRevisionNumber() error = %v, want ErrActiveConfigNotFound", err)
	}
}

func TestConfigRepositoryListMethodsReturnNotFoundWithoutWorkingRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	tests := []struct {
		name string
		run  func(context.Context) error
	}{
		{
			name: "policy bundles",
			run: func(ctx context.Context) error {
				_, err := testListPolicyBundles(ctx, repo)
				return err
			},
		},
		{
			name: "routes",
			run: func(ctx context.Context) error {
				_, err := testListRoutes(ctx, repo)
				return err
			},
		},
		{
			name: "providers",
			run: func(ctx context.Context) error {
				_, err := testListProviders(ctx, repo)
				return err
			},
		},
		{
			name: "model mappings",
			run: func(ctx context.Context) error {
				_, err := testListModelMappings(ctx, repo)
				return err
			},
		},
		{
			name: "verdict providers",
			run: func(ctx context.Context) error {
				_, err := testListVerdictProviders(ctx, repo)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(ctx); !errors.Is(err, ErrActiveConfigNotFound) {
				t.Fatalf("list error = %v, want ErrActiveConfigNotFound", err)
			}
		})
	}
}

func TestRemapIDHelpers(t *testing.T) {
	ids := map[string]string{"old-id": "new-id"}

	got, err := remapRequiredID("old-id", ids, "provider")
	if err != nil {
		t.Fatalf("remapRequiredID() error = %v", err)
	}
	if got != "new-id" {
		t.Fatalf("remapRequiredID() = %q, want new-id", got)
	}

	got, err = remapOptionalID("", ids, "optional provider")
	if err != nil {
		t.Fatalf("remapOptionalID(empty) error = %v", err)
	}
	if got != "" {
		t.Fatalf("remapOptionalID(empty) = %q, want empty", got)
	}

	got, err = remapOptionalID("old-id", ids, "optional provider")
	if err != nil {
		t.Fatalf("remapOptionalID(mapped) error = %v", err)
	}
	if got != "new-id" {
		t.Fatalf("remapOptionalID(mapped) = %q, want new-id", got)
	}

	_, err = remapRequiredID("missing-id", ids, "policy rule")
	if err == nil {
		t.Fatal("remapRequiredID(missing) error = nil, want error")
	}
	if err.Error() != "clone active config: policy rule missing-id was not cloned" {
		t.Fatalf("remapRequiredID(missing) error = %v, want cloned ID context", err)
	}

	_, err = remapOptionalID("missing-id", ids, "optional route")
	if err == nil {
		t.Fatal("remapOptionalID(missing) error = nil, want error")
	}
	if err.Error() != "clone active config: optional route missing-id was not cloned" {
		t.Fatalf("remapOptionalID(missing) error = %v, want cloned ID context", err)
	}
}

func TestDurationMillis(t *testing.T) {
	if got := durationMillis(1500 * time.Millisecond); got != 1500 {
		t.Fatalf("durationMillis(1500ms) = %d, want 1500", got)
	}
	if got := durationMillis(0); got != 0 {
		t.Fatalf("durationMillis(0) = %d, want 0", got)
	}
}
