package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestConfigRepositoryActivatesPolicyBundlesAndRecordsSnapshot(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	bundle := policystore.Bundle{
		Key:           "pii",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
		Detectors: []policystore.Detector{{
			Key:        "email",
			Kind:       string(detection.KindRegex),
			Pattern:    "[a-z]+@[a-z]+\\.com",
			Categories: []string{"pii.email"},
			Enabled:    true,
		}},
		Rules: []policystore.Rule{{
			Key:          "block-email",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"email"},
			Scopes:       []policystore.RuleScope{{Direction: string(detection.DirectionInput)}},
		}},
	}
	if err := testUpsertPolicyBundle(ctx, repo, bundle); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}
	listed, err := testListPolicyBundles(ctx, repo)
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Key != "pii" || listed[0].Detectors[0].Kind != string(detection.KindRegex) {
		t.Fatalf("ListPolicyBundles() = %+v, want pii regex bundle", listed)
	}

	request := activationRequestForRepo(t, ctx, repo, []string{"pii"})
	request.Actor = "alice"
	request.Reason = "initial activation"
	result, err := testActivatePolicyBundles(ctx, repo, request)
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	if result.RevisionNumber == 0 {
		t.Fatal("RevisionNumber = 0, want non-zero")
	}
	if result.SnapshotHash == "" {
		t.Fatal("SnapshotHash = empty, want compiled hash")
	}
	if len(result.ActiveKeys) != 1 || result.ActiveKeys[0] != "pii" {
		t.Fatalf("ActiveKeys = %+v, want [pii]", result.ActiveKeys)
	}

	active, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() error = %v", err)
	}
	if active != result.RevisionNumber {
		t.Fatalf("ActiveRevisionNumber() = %d, want %d", active, result.RevisionNumber)
	}

	var snapshotCount, activationCount int
	err = pool.QueryRow(ctx, `select count(*) from compiled_snapshots`).Scan(&snapshotCount)
	if err != nil {
		t.Fatalf("query compiled_snapshots: %v", err)
	}
	err = pool.QueryRow(ctx, `select count(*) from policy_activation_records where actor = 'alice' and status = 'active'`).Scan(&activationCount)
	if err != nil {
		t.Fatalf("query policy_activation_records: %v", err)
	}
	if snapshotCount != 1 || activationCount != 1 {
		t.Fatalf("snapshotCount=%d activationCount=%d, want 1 and 1", snapshotCount, activationCount)
	}
}

func TestConfigRepositoryActivationReturnsRuntimeRevisionTokenAfterGatewayClientMutation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "enabled",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
	}); err != nil {
		t.Fatalf("UpsertGatewayClient() error = %v", err)
	}
	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "pii",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}

	result, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii"}))
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	active, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() error = %v", err)
	}
	if result.RevisionNumber != active {
		t.Fatalf("Activation RevisionNumber = %d, want active runtime token %d", result.RevisionNumber, active)
	}
}

func TestConfigRepositoryPersistsPolicyBundleDefaultAction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "strict",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionBlock),
		Enabled:       true,
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}

	listed, err := testListPolicyBundles(ctx, repo)
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if len(listed) != 1 || listed[0].DefaultAction != string(policy.ActionBlock) {
		t.Fatalf("DefaultAction = %+v, want block", listed)
	}
}

func TestConfigRepositoryActivationDisablesUnrequestedDraftBundles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	for _, key := range []string{"pii", "secrets"} {
		if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
			Key:           key,
			Version:       "2026.05",
			Source:        string(policy.SourceUser),
			DefaultAction: string(policy.ActionAllow),
			Enabled:       true,
		}); err != nil {
			t.Fatalf("UpsertPolicyBundle(%s) error = %v", key, err)
		}
	}

	if _, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii"})); err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if len(cfg.PolicyBundles) != 2 {
		t.Fatalf("PolicyBundles len = %d, want both stored bundles", len(cfg.PolicyBundles))
	}
	states := make(map[string]bool, len(cfg.PolicyBundles))
	for _, bundle := range cfg.PolicyBundles {
		states[bundle.Key] = bundle.Enabled
	}
	if !states["pii"] || states["secrets"] {
		t.Fatalf("bundle enabled states = %+v, want only pii active", states)
	}
}

func TestConfigRepositoryUpsertsPolicyRuleRouteProviderScopes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	routes, err := testListRoutes(ctx, repo)
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	providers, err := testListProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(routes) != 1 || len(providers) != 1 {
		t.Fatalf("routes/providers = %d/%d, want 1/1", len(routes), len(providers))
	}

	bundle := policystore.Bundle{
		Key:           "scoped",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
		Detectors: []policystore.Detector{{
			Key:     "email",
			Kind:    string(detection.KindRegex),
			Pattern: "[a-z]+@[a-z]+\\.com",
			Enabled: true,
		}},
		Rules: []policystore.Rule{{
			Key:          "block-chat-email",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"email"},
			Scopes: []policystore.RuleScope{{
				RouteID:    routes[0].ID,
				ProviderID: providers[0].ID,
				Model:      "gpt-test",
				Direction:  string(detection.DirectionInput),
			}},
		}},
	}
	if err := testUpsertPolicyBundle(ctx, repo, bundle); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}
	bundles, err := testListPolicyBundles(ctx, repo)
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	var scoped policystore.Bundle
	for _, item := range bundles {
		if item.Key == "scoped" {
			scoped = item
			break
		}
	}
	if len(scoped.Rules) != 1 || len(scoped.Rules[0].Scopes) != 1 {
		t.Fatalf("scoped bundle = %+v, want one scoped rule", scoped)
	}
	scope := scoped.Rules[0].Scopes[0]
	if scope.RouteID != routes[0].ID || scope.ProviderID != providers[0].ID || scope.Direction != string(detection.DirectionInput) {
		t.Fatalf("scope = %+v, want route/provider IDs and input direction", scope)
	}
}

func TestConfigRepositoryActivatePolicyBundlesReturnsErrorForMissingBundle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "pii",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}

	_, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii", "missing"}))
	if err == nil || err.Error() != "activate policy bundles: requested bundle not found" {
		t.Fatalf("ActivatePolicyBundles() error = %v, want missing bundle error", err)
	}
	if _, err := repo.ActiveRevisionNumber(ctx); !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("ActiveRevisionNumber() error = %v, want ErrActiveConfigNotFound after failed activation", err)
	}
}

func TestConfigRepositoryActivatePolicyBundlesLoadsOnlyRequestedBundles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "pii",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
		Detectors: []policystore.Detector{{
			Key:     "email",
			Kind:    string(detection.KindRegex),
			Pattern: "[a-z]+@[a-z]+\\.com",
			Enabled: true,
		}},
		Rules: []policystore.Rule{{
			Key:          "block-email",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"email"},
		}},
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle(pii) error = %v", err)
	}
	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "experimental",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       false,
		Detectors: []policystore.Detector{{
			Key:     "broken-regex",
			Kind:    string(detection.KindRegex),
			Pattern: "[",
			Enabled: true,
		}},
		Rules: []policystore.Rule{{
			Key:          "would-fail-if-compiled",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"broken-regex"},
		}},
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle(experimental) error = %v", err)
	}

	result, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii"}))
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	if len(result.ActiveKeys) != 1 || result.ActiveKeys[0] != "pii" {
		t.Fatalf("ActiveKeys = %+v, want [pii]", result.ActiveKeys)
	}
}

func TestLoadPolicyBundlesByKeysUsesBatchHydrationQueries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	for _, key := range []string{"pii", "secrets"} {
		if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
			Key:           key,
			Version:       "2026.05",
			Source:        string(policy.SourceUser),
			DefaultAction: string(policy.ActionAllow),
			Enabled:       true,
			Detectors: []policystore.Detector{{
				Key:     key + "-email",
				Kind:    string(detection.KindRegex),
				Pattern: "[a-z]+@[a-z]+\\.com",
				Enabled: true,
			}},
			Rules: []policystore.Rule{{
				Key:          key + "-rule",
				Enabled:      true,
				Severity:     string(policy.SeverityHigh),
				Action:       string(policy.ActionBlock),
				DetectorKeys: []string{key + "-email"},
			}},
		}); err != nil {
			t.Fatalf("UpsertPolicyBundle(%s) error = %v", key, err)
		}
	}

	var bundles []policystore.Bundle
	counted := &countingPolicyBundleQueryer{}
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		counted.delegate = q
		var err error
		bundles, err = policystore.LoadBundlesByKeys(ctx, counted, revisionID, []string{"pii", "secrets"})
		return err
	})
	if err != nil {
		t.Fatalf("WithCurrentRevision() error = %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("bundle count = %d, want 2", len(bundles))
	}
	if counted.legacyHydrationQueries != 0 {
		t.Fatalf("legacy hydration query count = %d, want batched hydration without per-bundle/per-rule lookups", counted.legacyHydrationQueries)
	}
}

func TestConfigRepositoryActivationDefaultsActorToSystem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "pii",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}
	if _, err := testActivatePolicyBundles(ctx, repo, activationRequestForRepo(t, ctx, repo, []string{"pii"})); err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}

	var actor string
	err := pool.QueryRow(ctx, `select actor from policy_activation_records where status = 'active'`).Scan(&actor)
	if err != nil {
		t.Fatalf("query policy_activation_records actor: %v", err)
	}
	if actor != "system" {
		t.Fatalf("actor = %q, want system", actor)
	}
}

func TestConfigRepositoryUpsertPolicyBundleRejectsUnknownRuleDetector(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "broken",
		Version:       "2026.05",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
		Rules: []policystore.Rule{{
			Key:          "references-missing-detector",
			Enabled:      true,
			Severity:     string(policy.SeverityHigh),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"missing"},
		}},
	})
	if err == nil || err.Error() != "insert policy rule detector: detector missing not found" {
		t.Fatalf("UpsertPolicyBundle() error = %v, want missing detector error", err)
	}
	if _, err := testListPolicyBundles(ctx, repo); !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("ListPolicyBundles() error = %v, want ErrActiveConfigNotFound after rollback", err)
	}
}

func TestDetectorConfigPreservesBuiltInDetectorKind(t *testing.T) {
	config := detectorConfig("email", []string{"pii.email"})

	if got := detectorPolicyKind("builtin", config); got != "email" {
		t.Fatalf("detectorPolicyKind() = %q, want email", got)
	}
	if got := detectorStorageKind("email"); got != "builtin" {
		t.Fatalf("detectorStorageKind(email) = %q, want builtin", got)
	}
	if got := detectorStorageKind("regex"); got != "regex" {
		t.Fatalf("detectorStorageKind(regex) = %q, want regex", got)
	}
}

func TestToPolicyBundlesSkipsInactiveBundlesAndDetectors(t *testing.T) {
	bundles := toPolicyBundles([]policystore.Bundle{
		{Key: "disabled", Version: "1", Source: string(policy.SourceUser), Enabled: false},
		{
			Key:           "active",
			Version:       "1",
			Source:        string(policy.SourceUser),
			DefaultAction: "",
			Enabled:       true,
			Detectors: []policystore.Detector{
				{Key: "disabled-detector", Kind: string(detection.KindEmail), Enabled: false},
				{Key: "email", Kind: string(detection.KindEmail), Categories: []string{"pii.email"}, Enabled: true},
			},
			Rules: []policystore.Rule{{
				Key:          "shadow-email",
				Enabled:      true,
				Severity:     string(policy.SeverityMedium),
				Action:       string(policy.ActionShadowLog),
				DetectorKeys: []string{"email"},
				Scopes:       []policystore.RuleScope{{Model: "gpt-4o-mini", Direction: string(detection.DirectionInput)}},
			}},
		},
	})

	if len(bundles) != 1 || bundles[0].Key != "active" {
		t.Fatalf("toPolicyBundles() = %+v, want only active bundle", bundles)
	}
	bundle := bundles[0]
	if bundle.DefaultAction != policy.ActionAllow {
		t.Fatalf("DefaultAction = %q, want allow fallback", bundle.DefaultAction)
	}
	if len(bundle.Detectors) != 1 || bundle.Detectors[0].Key != "email" {
		t.Fatalf("Detectors = %+v, want only enabled detector", bundle.Detectors)
	}
	if len(bundle.Rules) != 1 || bundle.Rules[0].Scope.Model != "gpt-4o-mini" || bundle.Rules[0].Scope.Direction != detection.DirectionInput {
		t.Fatalf("Rules = %+v, want scoped rule", bundle.Rules)
	}
}

func TestDetectorHelpersHandleFallbacksAndMalformedConfig(t *testing.T) {
	if got := detectorStorageKind("custom"); got != "custom" {
		t.Fatalf("detectorStorageKind(custom) = %q, want custom", got)
	}
	if got := detectorPolicyKind("regex", json.RawMessage(`{"kind":"email"}`)); got != "regex" {
		t.Fatalf("detectorPolicyKind(regex) = %q, want regex", got)
	}
	if got := detectorPolicyKind("builtin", json.RawMessage(`{bad-json}`)); got != "builtin" {
		t.Fatalf("detectorPolicyKind(malformed) = %q, want builtin fallback", got)
	}
	if got := detectorPolicyKind("builtin", json.RawMessage(`{}`)); got != "builtin" {
		t.Fatalf("detectorPolicyKind(empty) = %q, want builtin fallback", got)
	}
	if got := detectorCategories(json.RawMessage(`{"categories":["pii.email","secret"]}`)); len(got) != 2 || got[0] != "pii.email" || got[1] != "secret" {
		t.Fatalf("detectorCategories() = %+v, want configured categories", got)
	}
	if got := detectorCategories(json.RawMessage(`{bad-json}`)); got != nil {
		t.Fatalf("detectorCategories(malformed) = %+v, want nil", got)
	}
}

func TestDirectionAndDefaultJSONHelpers(t *testing.T) {
	tests := []struct {
		input       string
		wantStore   string
		wantRuntime string
	}{
		{input: string(detection.DirectionInput), wantStore: "request"},
		{input: "request", wantStore: "request", wantRuntime: string(detection.DirectionInput)},
		{input: string(detection.DirectionOutput), wantStore: "response"},
		{input: "response", wantStore: "response", wantRuntime: string(detection.DirectionOutput)},
		{input: "both", wantStore: "both"},
		{input: "unknown", wantStore: "both"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := storageDirection(tt.input); got != tt.wantStore {
				t.Fatalf("storageDirection(%q) = %q, want %q", tt.input, got, tt.wantStore)
			}
			if tt.wantRuntime != "" {
				if got := policyDirection(tt.input); got != tt.wantRuntime {
					t.Fatalf("policyDirection(%q) = %q, want %q", tt.input, got, tt.wantRuntime)
				}
			}
		})
	}
	if got := policyDirection("both"); got != "" {
		t.Fatalf("policyDirection(both) = %q, want empty wildcard", got)
	}
	if got := string(defaultJSONObject(nil)); got != "{}" {
		t.Fatalf("defaultJSONObject(nil) = %q, want {}", got)
	}
	if got := string(defaultJSONObject(json.RawMessage(`{"a":1}`))); got != `{"a":1}` {
		t.Fatalf("defaultJSONObject(raw) = %q, want original JSON", got)
	}
}

func activationRequestForRepo(t *testing.T, ctx context.Context, repo *ConfigRepository, keys []string) policystore.ActivationRequest {
	t.Helper()
	bundles, err := testListPolicyBundles(ctx, repo)
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	byKey := make(map[string]policystore.Bundle, len(bundles))
	for _, bundle := range bundles {
		byKey[bundle.Key] = bundle
	}
	selected := make([]policystore.Bundle, 0, len(keys))
	for _, key := range keys {
		if bundle, ok := byKey[key]; ok {
			selected = append(selected, bundle)
		}
	}
	snapshot, err := policy.CompileSnapshot(toPolicyBundles(selected))
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}
	return policystore.ActivationRequest{
		Keys:         append([]string(nil), keys...),
		SnapshotHash: snapshot.Hash(),
	}
}

type countingPolicyBundleQueryer struct {
	delegate               queryer
	legacyHydrationQueries int
}

func (q *countingPolicyBundleQueryer) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	normalized := strings.Join(strings.Fields(sql), " ")
	if strings.Contains(normalized, "from policy_detectors") && strings.Contains(normalized, "where bundle_id = $1") {
		q.legacyHydrationQueries++
	}
	if strings.Contains(normalized, "from policy_rules") && strings.Contains(normalized, "where bundle_id = $1") {
		q.legacyHydrationQueries++
	}
	if strings.Contains(normalized, "from policy_rule_detectors") && strings.Contains(normalized, "where rd.rule_id = $1") {
		q.legacyHydrationQueries++
	}
	if strings.Contains(normalized, "from policy_rule_scopes") && strings.Contains(normalized, "where rule_id = $1") {
		q.legacyHydrationQueries++
	}
	return q.delegate.Query(ctx, sql, args...)
}

func (q *countingPolicyBundleQueryer) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return q.delegate.QueryRow(ctx, sql, args...)
}

func (q *countingPolicyBundleQueryer) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return q.delegate.Exec(ctx, sql, args...)
}
