package configstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
	"github.com/jackc/pgx/v5"
)

func TestConfigRepositoryOrchestratesConfigurationPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)
	seedOpenAIRoute(t, ctx, pool)
	seedGatewayClientAndLimits(t, ctx, pool)

	active, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if active.Revision.Number == 0 || len(active.Routes) != 2 || len(active.Providers) != 1 {
		t.Fatalf("active runtime config = %+v, want seeded revision graph", active)
	}

	routes, err := testListRoutes(ctx, repo)
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("ListRoutes() len = %d, want 2", len(routes))
	}
	routeID := routeIDByName(t, ctx, repo, "openai")

	providers, err := testListProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("ListProviders() = %+v, want seeded provider", providers)
	}

	if err := testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{
		Name:             "console-default",
		SourceModel:      "gpt-console",
		TargetProviderID: providers[0].ID,
		TargetModel:      "gpt-4o-mini",
	}); err != nil {
		t.Fatalf("UpsertModelMapping() error = %v", err)
	}
	mappings, err := testListModelMappings(ctx, repo)
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if !containsModelMapping(mappings, "console-default") {
		t.Fatalf("ListModelMappings() = %+v, want console-default", mappings)
	}

	if err := testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
		Name:           "shadow-sec-model",
		Endpoint:       "http://verdict.test/shadow",
		CredentialRef:  "secret/shadow",
		ModelName:      "kg-shadow",
		MaxConcurrency: 4,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("UpsertVerdictProvider() error = %v", err)
	}
	verdictProviders, err := testListVerdictProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if !containsVerdictProvider(verdictProviders, "shadow-sec-model") {
		t.Fatalf("ListVerdictProviders() = %+v, want shadow-sec-model", verdictProviders)
	}

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "secrets",
		Version:       "2026.06",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionBlock),
		Enabled:       true,
		Detectors: []policystore.Detector{{
			Key:        "token",
			Kind:       string(detection.KindRegex),
			Pattern:    "sk-[A-Za-z0-9]{8,}",
			Categories: []string{"secret.token"},
			Enabled:    true,
		}},
		Rules: []policystore.Rule{{
			Key:          "block-token",
			Enabled:      true,
			Severity:     string(policy.SeverityCritical),
			Action:       string(policy.ActionBlock),
			DetectorKeys: []string{"token"},
			Scopes:       []policystore.RuleScope{{Direction: string(detection.DirectionInput)}},
		}},
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}
	bundles, err := testListPolicyBundles(ctx, repo)
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if !containsPolicyBundle(bundles, "secrets") {
		t.Fatalf("ListPolicyBundles() = %+v, want secrets bundle", bundles)
	}

	if err := testCreateGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-beta",
		Name:       "Beta Console",
		KeyPrefix:  "kg_beta",
		KeyHash:    "sha256:beta",
		Notes:      "repository orchestration test",
	}); err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}
	if err := testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-beta",
		Name:       "Beta Console Updated",
		KeyPrefix:  "kg_beta",
		KeyHash:    "sha256:beta-updated",
		Notes:      "repository orchestration update",
	}); err != nil {
		t.Fatalf("UpsertGatewayClient() error = %v", err)
	}
	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if !containsGatewayClient(clients, "client-beta", "Beta Console Updated") {
		t.Fatalf("ListGatewayClients() = %+v, want updated beta client", clients)
	}
	if err := testRevokeGatewayClient(ctx, repo, "client-beta"); err != nil {
		t.Fatalf("RevokeGatewayClient() error = %v", err)
	}

	if err := testUpsertRouteLimitPolicy(ctx, repo, limitstore.RoutePolicy{
		RouteID:               routeID,
		RequestsPerWindow:     240,
		WindowSeconds:         60,
		MaxConcurrentRequests: 12,
		MaxBodyBytes:          2 << 20,
		Enabled:               true,
	}); err != nil {
		t.Fatalf("UpsertRouteLimitPolicy() error = %v", err)
	}
	routeLimits, err := testListRouteLimitPolicies(ctx, repo)
	if err != nil {
		t.Fatalf("ListRouteLimitPolicies() error = %v", err)
	}
	if !containsRouteLimit(routeLimits, 240) {
		t.Fatalf("ListRouteLimitPolicies() = %+v, want updated 240 rpm limit", routeLimits)
	}

	clients, err = testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() after revoke error = %v", err)
	}
	clientID := clientIDByExternalID(t, clients, "client-acme")
	if err := testUpsertClientRouteLimitOverride(ctx, repo, limitstore.ClientRouteOverride{
		ClientID:              clientID,
		RouteID:               routeID,
		RequestsPerWindow:     55,
		WindowSeconds:         60,
		MaxConcurrentRequests: 5,
		MaxBodyBytes:          512 << 10,
		Enabled:               true,
	}); err != nil {
		t.Fatalf("UpsertClientRouteLimitOverride() error = %v", err)
	}
	clientLimits, err := testListClientRouteLimitOverrides(ctx, repo, clientID)
	if err != nil {
		t.Fatalf("ListClientRouteLimitOverrides() error = %v", err)
	}
	if !containsClientRouteLimit(clientLimits, 55) {
		t.Fatalf("ListClientRouteLimitOverrides() = %+v, want updated client limit", clientLimits)
	}
	if err := testDeleteClientRouteLimitOverride(ctx, repo, clientID, routeID); err != nil {
		t.Fatalf("DeleteClientRouteLimitOverride() error = %v", err)
	}
}

func TestConfigRepositoryLoadActiveRuntimeConfigReportsBeginError(t *testing.T) {
	ctx := context.Background()
	repo := &ConfigRepository{pool: &fakeConfigDB{beginErr: errors.New("db unavailable")}}

	_, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err == nil || !strings.Contains(err.Error(), "begin active revision transaction") {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v, want begin transaction context", err)
	}
}

func TestConfigRepositoryLoadActiveRuntimeConfigCommitsEmptyGraph(t *testing.T) {
	ctx := context.Background()
	tx := fakeLoadActiveRuntimeConfigTx(nil)
	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if !tx.committed {
		t.Fatal("LoadActiveRuntimeConfig() did not commit read transaction")
	}
	if tx.rollbacks != 1 {
		t.Fatalf("rollback calls = %d, want deferred rollback after commit", tx.rollbacks)
	}
	if cfg.Revision.ID != "revision-1" || cfg.Revision.Number != 42 {
		t.Fatalf("Revision = %+v, want revision-1 number 42", cfg.Revision)
	}
	if len(cfg.Routes) != 0 || len(cfg.PolicyBundles) != 0 || len(cfg.GatewayClients) != 0 {
		t.Fatalf("active graph = %+v, want empty loaded graph", cfg)
	}
}

func TestConfigRepositoryLoadActiveRuntimeConfigReportsCommitError(t *testing.T) {
	ctx := context.Background()
	tx := fakeLoadActiveRuntimeConfigTx(errors.New("commit failed"))
	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}

	_, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err == nil || !strings.Contains(err.Error(), "commit active revision transaction") {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v, want commit transaction context", err)
	}
	if !tx.committed {
		t.Fatal("LoadActiveRuntimeConfig() did not attempt commit")
	}
}

func TestConfigRepositoryMutationMethodsReportBeginErrors(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		run  func(context.Context, *ConfigRepository) error
	}{
		{
			name: "upsert policy bundle",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertPolicyBundle(ctx, repo, policystore.Bundle{Key: "pii"})
			},
		},
		{
			name: "activate policy bundles",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				_, err := testActivatePolicyBundles(ctx, repo, policystore.ActivationRequest{Keys: []string{"pii"}, SnapshotHash: "hash"})
				return err
			},
		},
		{
			name: "upsert model mapping",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{Name: "default"})
			},
		},
		{
			name: "upsert verdict provider",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{Name: "verdict"})
			},
		},
		{
			name: "create gateway client",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testCreateGatewayClient(ctx, repo, clientstore.GatewayClient{ExternalID: "client"})
			},
		},
		{
			name: "upsert gateway client",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{ExternalID: "client"})
			},
		},
		{
			name: "revoke gateway client",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testRevokeGatewayClient(ctx, repo, "client")
			},
		},
		{
			name: "upsert route limit",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertRouteLimitPolicy(ctx, repo, limitstore.RoutePolicy{RouteID: "route"})
			},
		},
		{
			name: "upsert client route limit",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertClientRouteLimitOverride(ctx, repo, limitstore.ClientRouteOverride{ClientID: "client", RouteID: "route"})
			},
		},
		{
			name: "delete client route limit",
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testDeleteClientRouteLimitOverride(ctx, repo, "client", "route")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &ConfigRepository{pool: &fakeConfigDB{beginErr: errors.New("db unavailable")}}
			if err := tt.run(ctx, repo); err == nil || !strings.Contains(err.Error(), "transaction") {
				t.Fatalf("mutation error = %v, want transaction begin context", err)
			}
		})
	}
}

func TestConfigRepositoryMutationMethodsCommitTransactions(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		tx      *fakeConfigTx
		run     func(context.Context, *ConfigRepository) error
		wantSQL []string
	}{
		{
			name: "upsert policy bundle",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{
					rowWithConfigValues("draft-1"),
					rowWithConfigValues("bundle-1"),
					rowWithConfigValues("detector-1"),
					rowWithConfigValues("rule-1"),
				},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
					Key:     "secrets",
					Version: "v1",
					Enabled: true,
					Detectors: []policystore.Detector{{
						Key:     "token",
						Kind:    string(detection.KindRegex),
						Pattern: "sk-[A-Za-z0-9]+",
						Enabled: true,
					}},
					Rules: []policystore.Rule{{
						Key:          "block-token",
						Enabled:      true,
						Action:       string(policy.ActionBlock),
						DetectorKeys: []string{"token"},
						Scopes:       []policystore.RuleScope{{Direction: string(detection.DirectionInput)}},
					}},
				})
			},
			wantSQL: []string{"policy_bundles", "policy_detectors", "policy_rules"},
		},
		{
			name: "upsert model mapping",
			tx:   &fakeConfigTx{rowResults: []fakeConfigRow{rowWithConfigValues("draft-1")}},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{
					Name:        "default",
					SourceModel: "gpt-source",
					TargetModel: "gpt-target",
				})
			},
			wantSQL: []string{"model_mappings"},
		},
		{
			name: "upsert verdict provider",
			tx:   &fakeConfigTx{rowResults: []fakeConfigRow{rowWithConfigValues("draft-1")}},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
					Name:     "security-model",
					Endpoint: "http://verdict.test/evaluate",
					Enabled:  true,
				})
			},
			wantSQL: []string{"verdict_providers"},
		},
		{
			name: "create gateway client",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{rowWithConfigValues(int64(7))},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testCreateGatewayClient(ctx, repo, clientstore.GatewayClient{
					ExternalID: "client-a",
					Name:       "Client A",
					KeyPrefix:  "kg_a",
					KeyHash:    "sha256:a",
				})
			},
			wantSQL: []string{"gateway_clients", "gateway_client_config_versions", "pg_notify"},
		},
		{
			name: "upsert gateway client",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{rowWithConfigValues(int64(8))},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
					ExternalID: "client-b",
					Name:       "Client B",
					KeyPrefix:  "kg_b",
					KeyHash:    "sha256:b",
				})
			},
			wantSQL: []string{"gateway_clients", "gateway_client_config_versions", "pg_notify"},
		},
		{
			name: "revoke gateway client without generation bump",
			tx:   &fakeConfigTx{},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testRevokeGatewayClient(ctx, repo, "client-c")
			},
			wantSQL: []string{"gateway_clients"},
		},
		{
			name: "upsert route limit policy",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{
					rowWithConfigValues("draft-1"),
					rowWithConfigValues("route-1"),
				},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertRouteLimitPolicy(ctx, repo, limitstore.RoutePolicy{
					RouteID:               "route-1",
					RequestsPerWindow:     100,
					WindowSeconds:         60,
					MaxConcurrentRequests: 8,
					MaxBodyBytes:          1024,
					Enabled:               true,
				})
			},
			wantSQL: []string{"route_limit_policies"},
		},
		{
			name: "upsert client route limit override",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{
					rowWithConfigValues("draft-1"),
					rowWithConfigValues("route-1"),
				},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testUpsertClientRouteLimitOverride(ctx, repo, limitstore.ClientRouteOverride{
					ClientID:              "client-1",
					RouteID:               "route-1",
					RequestsPerWindow:     25,
					WindowSeconds:         60,
					MaxConcurrentRequests: 4,
					MaxBodyBytes:          1024,
					Enabled:               true,
				})
			},
			wantSQL: []string{"client_route_limit_overrides"},
		},
		{
			name: "delete client route limit override",
			tx: &fakeConfigTx{
				rowResults: []fakeConfigRow{
					rowWithConfigValues("draft-1"),
					rowWithConfigValues("route-1"),
				},
			},
			run: func(ctx context.Context, repo *ConfigRepository) error {
				return testDeleteClientRouteLimitOverride(ctx, repo, "client-1", "route-1")
			},
			wantSQL: []string{"client_route_limit_overrides"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &ConfigRepository{pool: &fakeConfigDB{tx: tt.tx}}
			if err := tt.run(ctx, repo); err != nil {
				t.Fatalf("mutation error = %v", err)
			}
			if !tt.tx.committed {
				t.Fatal("mutation did not commit transaction")
			}
			if tt.tx.rollbacks != 1 {
				t.Fatalf("rollback calls = %d, want deferred rollback after commit", tt.tx.rollbacks)
			}
			for _, want := range tt.wantSQL {
				if countQueries(tt.tx.execSQL, want) == 0 && countQueries(tt.tx.rowSQL, want) == 0 {
					t.Fatalf("SQL calls did not include %q; rows=%#v execs=%#v", want, tt.tx.rowSQL, tt.tx.execSQL)
				}
			}
		})
	}
}

func TestEnsureDraftRevisionCreatesEmptyDraftWithoutActiveRevision(t *testing.T) {
	ctx := context.Background()
	tx := &fakeConfigTx{
		rowResults: []fakeConfigRow{
			rowWithConfigErr(pgx.ErrNoRows),
			rowWithConfigErr(pgx.ErrNoRows),
			rowWithConfigValues("draft-1"),
		},
	}

	repo := &ConfigRepository{pool: &fakeConfigDB{tx: tx}}
	var id string
	err := repo.WithDraftRevision(ctx, "create empty draft", func(_ context.Context, _ revisionstore.Queryer, revisionID string) error {
		id = revisionID
		return nil
	})
	if err != nil {
		t.Fatalf("WithDraftRevision() error = %v", err)
	}
	if id != "draft-1" {
		t.Fatalf("WithDraftRevision() revision = %q, want draft-1", id)
	}
	if countQueries(tx.rowSQL, "insert into config_revisions") != 1 {
		t.Fatalf("row SQL calls = %#v, want empty draft insert", tx.rowSQL)
	}
}

func fakeLoadActiveRuntimeConfigTx(commitErr error) *fakeConfigTx {
	return &fakeConfigTx{
		commitErr: commitErr,
		rowResults: []fakeConfigRow{
			rowWithConfigValues("revision-1", int64(42), "test", "active", "system", "hash", "ref", time.Unix(1, 0).UTC()),
		},
		rows: []*fakeConfigRows{
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
			newConfigRows(nil),
		},
	}
}

func containsModelMapping(mappings []routingstore.ModelMapping, name string) bool {
	for _, mapping := range mappings {
		if mapping.Name == name {
			return true
		}
	}
	return false
}

func containsVerdictProvider(providers []routingstore.VerdictProvider, name string) bool {
	for _, provider := range providers {
		if provider.Name == name {
			return true
		}
	}
	return false
}

func containsPolicyBundle(bundles []policystore.Bundle, key string) bool {
	for _, bundle := range bundles {
		if bundle.Key == key {
			return true
		}
	}
	return false
}

func containsGatewayClient(clients []clientstore.GatewayClient, externalID, name string) bool {
	for _, client := range clients {
		if client.ExternalID == externalID && client.Name == name {
			return true
		}
	}
	return false
}

func containsRouteLimit(policies []limitstore.RoutePolicy, requestsPerWindow int) bool {
	for _, policy := range policies {
		if policy.RequestsPerWindow == requestsPerWindow {
			return true
		}
	}
	return false
}

func containsClientRouteLimit(overrides []limitstore.ClientRouteOverride, requestsPerWindow int) bool {
	for _, override := range overrides {
		if override.RequestsPerWindow == requestsPerWindow {
			return true
		}
	}
	return false
}

func clientIDByExternalID(t *testing.T, clients []clientstore.GatewayClient, externalID string) string {
	t.Helper()

	for _, client := range clients {
		if client.ExternalID == externalID {
			return client.ID
		}
	}
	t.Fatalf("client %q not found in %+v", externalID, clients)
	return ""
}
