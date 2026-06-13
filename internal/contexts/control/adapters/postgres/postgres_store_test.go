package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

func TestPostgresPolicyStoreReturnsEmptyStatusWithoutActiveConfig(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{loadActiveErr: configstore.ErrActiveConfigNotFound})

	status, err := store.ConfigStatus(context.Background())
	if err != nil {
		t.Fatalf("ConfigStatus() error = %v", err)
	}
	if len(status.ActivePolicyBundleKeys) != 0 {
		t.Fatalf("ActivePolicyBundleKeys = %+v, want empty", status.ActivePolicyBundleKeys)
	}
	if status.PolicySnapshotHash != "" {
		t.Fatalf("PolicySnapshotHash = %q, want empty", status.PolicySnapshotHash)
	}
}

func TestPostgresPolicyStoreActivatesPolicyBundles(t *testing.T) {
	repo := &fakeConfigRepository{
		activation: policystore.ActivationResult{
			RevisionNumber: 12,
			SnapshotHash:   "hash",
			ActiveKeys:     []string{"pii"},
		},
	}
	store := NewPolicyStore(repo)

	got, err := store.ActivatePolicyBundles(context.Background(), appcontrol.PolicyActivationRequest{
		Keys:         []string{"pii"},
		Reason:       "initial activation",
		SnapshotHash: "hash",
	})
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	if got.RevisionNumber != 12 || got.Hash != "hash" || len(got.ActiveKeys) != 1 || got.ActiveKeys[0] != "pii" {
		t.Fatalf("activation = %+v, want revision 12 hash pii", got)
	}
	if len(repo.activatedKeys) != 1 || repo.activatedKeys[0] != "pii" {
		t.Fatalf("activated keys = %+v, want [pii]", repo.activatedKeys)
	}
	if repo.activationReason != "initial activation" {
		t.Fatalf("activation reason = %q, want initial activation", repo.activationReason)
	}
	if repo.activationHash != "hash" {
		t.Fatalf("activation hash = %q, want hash", repo.activationHash)
	}
}

func TestPostgresPolicyStoreMapsControlResources(t *testing.T) {
	repo := &fakeConfigRepository{
		activeConfig: configSnapshot{
			Routes:        []routingstore.Route{{ID: "route-id", Name: "chat"}},
			Providers:     []routingstore.Provider{{ID: "provider-id", Name: "openai"}},
			PolicyBundles: []policystore.Bundle{postgresBundleFixture()},
		},
		routes: []routingstore.Route{{
			ID:   "route-id",
			Name: "chat",
		}},
		providers: []routingstore.Provider{{
			ID:   "provider-id",
			Name: "openai",
		}},
		bundles: []policystore.Bundle{postgresBundleFixture()},
		mappings: []routingstore.ModelMapping{{
			Name:        "default",
			SourceModel: "gpt-test",
			TargetModel: "gpt-upstream",
			Parameters:  json.RawMessage(`{"route_key":"chat","provider":"openai","enabled":true}`),
		}, {
			Name:        "legacy",
			SourceModel: "legacy",
			TargetModel: "legacy-upstream",
		}},
		verdictProviders: []routingstore.VerdictProvider{{
			Name:     "sec-model",
			Endpoint: "http://verdict.test/evaluate",
			Enabled:  true,
		}},
	}
	store := NewPolicyStore(repo)

	status, err := store.ConfigStatus(context.Background())
	if err != nil {
		t.Fatalf("ConfigStatus() error = %v", err)
	}
	if len(status.ActivePolicyBundleKeys) != 1 || status.ActivePolicyBundleKeys[0] != "pii" || status.PolicySnapshotHash == "" {
		t.Fatalf("status = %+v, want active pii with hash", status)
	}

	bundles, err := store.ListPolicyBundles(context.Background())
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if len(bundles) != 1 || bundles[0].Key != "pii" || bundles[0].Detectors[0].Kind != "regex" {
		t.Fatalf("bundles = %+v, want mapped pii regex bundle", bundles)
	}
	if bundles[0].Rules[0].Scope.RouteKey != "chat" || bundles[0].Rules[0].Scope.Provider != "openai" {
		t.Fatalf("bundle scope = %+v, want chat/openai scope", bundles[0].Rules[0].Scope)
	}

	if err := store.CreatePolicyBundle(context.Background(), bundles[0]); err != nil {
		t.Fatalf("CreatePolicyBundle() error = %v", err)
	}
	if repo.upsertedBundle.Key != "pii" || repo.upsertedBundle.Rules[0].Scopes[0].Direction != "input" || repo.upsertedBundle.Rules[0].Scopes[0].RouteID != "route-id" || repo.upsertedBundle.Rules[0].Scopes[0].ProviderID != "provider-id" {
		t.Fatalf("upserted bundle = %+v, want pii input chat/openai scope", repo.upsertedBundle)
	}

	mappings, err := store.ListModelMappings(context.Background())
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if len(mappings) != 2 || mappings[0].ID != "default" || mappings[0].RouteKey != "chat" || mappings[0].Provider != "openai" || mappings[0].Model != "gpt-upstream" || !mappings[0].Enabled {
		t.Fatalf("mappings = %+v, want default chat openai gpt-upstream enabled", mappings)
	}
	if !mappings[1].Enabled {
		t.Fatalf("legacy mapping = %+v, want enabled default", mappings[1])
	}
	if err := store.PutModelMapping(context.Background(), appcontrol.ModelMapping{ID: "new", RouteKey: "chat", Provider: "anthropic", Model: "gpt-new", Enabled: true}); err != nil {
		t.Fatalf("PutModelMapping() error = %v", err)
	}
	if repo.upsertedMapping.Name != "new" || repo.upsertedMapping.SourceModel != "gpt-new" || repo.upsertedMapping.TargetModel != "gpt-new" {
		t.Fatalf("upserted mapping = %+v, want new gpt-new source and target", repo.upsertedMapping)
	}
	var params modelMappingParams
	if err := json.Unmarshal(repo.upsertedMapping.Parameters, &params); err != nil {
		t.Fatalf("unmarshal upserted mapping parameters: %v", err)
	}
	if params.RouteKey != "chat" || params.Provider != "anthropic" || !params.Enabled {
		t.Fatalf("upserted mapping parameters = %+v, want chat anthropic enabled", params)
	}

	providers, err := store.ListVerdictProviders(context.Background())
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].ID != "sec-model" || providers[0].Mode != "inline" {
		t.Fatalf("providers = %+v, want sec-model inline", providers)
	}
	if err := store.PutVerdictProvider(context.Background(), appcontrol.VerdictProvider{ID: "sec-new", Endpoint: "http://new.test/evaluate", Enabled: true}); err != nil {
		t.Fatalf("PutVerdictProvider() error = %v", err)
	}
	if repo.upsertedVerdictProvider.Name != "sec-new" || repo.upsertedVerdictProvider.Endpoint == "" {
		t.Fatalf("upserted verdict provider = %+v, want sec-new endpoint", repo.upsertedVerdictProvider)
	}
}

func TestPostgresPolicyStoreMapsGatewayClientsAndLimits(t *testing.T) {
	repo := &fakeConfigRepository{
		routes: []routingstore.Route{{
			ID:   "route-id",
			Name: "chat",
		}},
		gatewayClients: []clientstore.GatewayClient{{
			ID:         "postgres-client-id",
			ExternalID: "client-acme",
			Name:       "Acme",
			Status:     "enabled",
			KeyPrefix:  "kgc_client-acme",
			KeyHash:    "sha256:existing",
			Notes:      "Existing account",
		}},
		routeLimits: []limitstore.RoutePolicy{{
			RouteID:               "route-id",
			RequestsPerWindow:     10,
			WindowSeconds:         60,
			MaxConcurrentRequests: 2,
			MaxBodyBytes:          4096,
			Enabled:               true,
		}},
		clientRouteLimits: []limitstore.ClientRouteOverride{{
			ClientID:              "postgres-client-id",
			RouteID:               "route-id",
			RequestsPerWindow:     3,
			WindowSeconds:         30,
			MaxConcurrentRequests: 1,
			MaxBodyBytes:          1024,
			Enabled:               true,
		}},
	}
	store := NewPolicyStore(repo)

	clients, err := store.ListGatewayClients(context.Background())
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 1 || clients[0].ID != "client-acme" || clients[0].KeyPrefix != "kgc_client-acme" || clients[0].Notes != "Existing account" {
		t.Fatalf("clients = %+v, want external client-acme with notes", clients)
	}

	created, err := store.CreateGatewayClient(context.Background(), appcontrol.CreateGatewayClientRequest{ID: "client-new", Name: "New Client", Status: "enabled", Notes: "New account"})
	if err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}
	if created.Client.ID != "client-new" || created.Client.KeyPrefix == "" || created.Client.Notes != "New account" || created.Key == "" {
		t.Fatalf("created = %+v, want client-new with notes and key material", created)
	}
	if repo.upsertedClient.ExternalID != "client-new" || repo.upsertedClient.KeyHash == "" || repo.upsertedClient.Notes != "New account" {
		t.Fatalf("upserted client = %+v, want stored notes and key hash", repo.upsertedClient)
	}

	patched, err := store.PatchGatewayClient(context.Background(), appcontrol.GatewayClient{ID: "client-acme", Name: "Acme Updated", Status: "disabled", Notes: "Needs review"})
	if err != nil {
		t.Fatalf("PatchGatewayClient() error = %v", err)
	}
	if patched.Name != "Acme Updated" || patched.Status != "disabled" || patched.Notes != "Needs review" || repo.upsertedClient.KeyHash != "sha256:existing" || repo.upsertedClient.Notes != "Needs review" {
		t.Fatalf("patched = %+v upserted=%+v, want disabled with notes and existing key material", patched, repo.upsertedClient)
	}

	revoked, err := store.RevokeGatewayClient(context.Background(), "client-acme")
	if err != nil {
		t.Fatalf("RevokeGatewayClient() error = %v", err)
	}
	if revoked.Status != "revoked" || repo.revokedClientID != "client-acme" {
		t.Fatalf("revoked = %+v revokedClientID=%q, want revoked client-acme", revoked, repo.revokedClientID)
	}

	routeLimits, err := store.ListRouteLimits(context.Background())
	if err != nil {
		t.Fatalf("ListRouteLimits() error = %v", err)
	}
	if len(routeLimits) != 1 || routeLimits[0].RouteKey != "chat" || routeLimits[0].RequestsPerWindow != 10 {
		t.Fatalf("routeLimits = %+v, want mapped chat limit", routeLimits)
	}
	savedRouteLimit, err := store.PutRouteLimit(context.Background(), appcontrol.RouteLimit{
		RouteKey:              "chat",
		RequestsPerWindow:     20,
		WindowSeconds:         120,
		MaxConcurrentRequests: 4,
		MaxBodyBytes:          8192,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("PutRouteLimit() error = %v", err)
	}
	if savedRouteLimit.RouteKey != "chat" || repo.upsertedRouteLimit.RouteID != "route-id" || repo.upsertedRouteLimit.RequestsPerWindow != 20 {
		t.Fatalf("savedRouteLimit = %+v upserted=%+v, want route-id policy", savedRouteLimit, repo.upsertedRouteLimit)
	}

	overrides, err := store.ListClientRouteLimits(context.Background(), "client-acme")
	if err != nil {
		t.Fatalf("ListClientRouteLimits() error = %v", err)
	}
	if len(overrides) != 1 || overrides[0].ClientID != "client-acme" || overrides[0].RouteKey != "chat" {
		t.Fatalf("overrides = %+v, want mapped Acme chat override", overrides)
	}
	savedOverride, err := store.PutClientRouteLimit(context.Background(), appcontrol.ClientRouteLimit{
		ClientID:              "client-acme",
		RouteKey:              "chat",
		RequestsPerWindow:     5,
		WindowSeconds:         45,
		MaxConcurrentRequests: 2,
		MaxBodyBytes:          2048,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("PutClientRouteLimit() error = %v", err)
	}
	if savedOverride.ClientID != "client-acme" || repo.upsertedClientRouteLimit.ClientID != "postgres-client-id" || repo.upsertedClientRouteLimit.RouteID != "route-id" {
		t.Fatalf("savedOverride = %+v upserted=%+v, want postgres IDs", savedOverride, repo.upsertedClientRouteLimit)
	}
	if err := store.DeleteClientRouteLimit(context.Background(), "client-acme", "chat"); err != nil {
		t.Fatalf("DeleteClientRouteLimit() error = %v", err)
	}
	if repo.deletedClientID != "postgres-client-id" || repo.deletedRouteID != "route-id" {
		t.Fatalf("deleted client/route = %q/%q, want postgres-client-id/route-id", repo.deletedClientID, repo.deletedRouteID)
	}
}

func TestPostgresPolicyStoreDoesNotReturnKeysForExistingRevokedClients(t *testing.T) {
	repo := &fakeConfigRepository{
		gatewayClients: []clientstore.GatewayClient{{
			ID:         "postgres-client-id",
			ExternalID: "client-revoked",
			Name:       "Revoked",
			Status:     "revoked",
			KeyPrefix:  "kgc_old",
			KeyHash:    "sha256:old",
		}},
	}
	store := NewPolicyStore(repo)

	_, err := store.CreateGatewayClient(context.Background(), appcontrol.CreateGatewayClientRequest{ID: "client-revoked", Name: "Revived", Status: "enabled"})
	if !errors.Is(err, errGatewayClientAlreadyExists) {
		t.Fatalf("CreateGatewayClient() error = %v, want errGatewayClientAlreadyExists", err)
	}

	patched, err := store.PatchGatewayClient(context.Background(), appcontrol.GatewayClient{ID: "client-revoked", Name: "Still Revoked", Status: "enabled"})
	if err != nil {
		t.Fatalf("PatchGatewayClient() error = %v", err)
	}
	if patched.Status != "revoked" {
		t.Fatalf("patched status = %q, want revoked persisted state", patched.Status)
	}
	if repo.upsertedClient.Status != "revoked" {
		t.Fatalf("upserted status = %q, want revoked preservation", repo.upsertedClient.Status)
	}
}

func TestPostgresPolicyStoreReportsMissingGatewayClient(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{})

	if _, err := store.PatchGatewayClient(context.Background(), appcontrol.GatewayClient{ID: "missing", Name: "Missing", Status: "enabled"}); !errors.Is(err, errGatewayClientNotFound) {
		t.Fatalf("PatchGatewayClient() error = %v, want errGatewayClientNotFound", err)
	}
	if _, err := store.RevokeGatewayClient(context.Background(), "missing"); !errors.Is(err, errGatewayClientNotFound) {
		t.Fatalf("RevokeGatewayClient() error = %v, want errGatewayClientNotFound", err)
	}
	if _, err := store.PutClientRouteLimit(context.Background(), appcontrol.ClientRouteLimit{ClientID: "missing", RouteKey: "chat", RequestsPerWindow: 1, WindowSeconds: 60, MaxConcurrentRequests: 1, MaxBodyBytes: 1024, Enabled: true}); !errors.Is(err, errGatewayClientNotFound) {
		t.Fatalf("PutClientRouteLimit() error = %v, want errGatewayClientNotFound", err)
	}
}

func TestPostgresPolicyStoreReturnsEmptyListsWithoutActiveConfig(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{loadActiveErr: configstore.ErrActiveConfigNotFound})

	bundles, err := store.ListPolicyBundles(context.Background())
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if len(bundles) != 0 {
		t.Fatalf("ListPolicyBundles() = %+v, want empty list", bundles)
	}

	mappings, err := store.ListModelMappings(context.Background())
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if len(mappings) != 0 {
		t.Fatalf("ListModelMappings() = %+v, want empty list", mappings)
	}

	providers, err := store.ListVerdictProviders(context.Background())
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("ListVerdictProviders() = %+v, want empty list", providers)
	}
}

func TestPostgresPolicyStoreReportsMalformedModelMappingParameters(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{
		mappings: []routingstore.ModelMapping{{
			Name:        "broken",
			TargetModel: "gpt-upstream",
			Parameters:  json.RawMessage(`{bad-json}`),
		}},
	})

	_, err := store.ListModelMappings(context.Background())
	if err == nil {
		t.Fatal("ListModelMappings() error = nil, want parameter decode error")
	}
	if !strings.Contains(err.Error(), "decode model mapping parameters") {
		t.Fatalf("ListModelMappings() error = %v, want decode context", err)
	}
}

func TestPostgresPolicyStoreCreatePolicyBundleRejectsUnknownScopeNames(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{
		routes:    []routingstore.Route{{ID: "route-id", Name: "chat"}},
		providers: []routingstore.Provider{{ID: "provider-id", Name: "openai"}},
	})

	err := store.CreatePolicyBundle(context.Background(), appcontrol.PolicyBundle{
		Key:           "pii",
		Version:       "1",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Rules: []appcontrol.Rule{{
			Key:      "route-miss",
			Enabled:  true,
			Severity: string(policy.SeverityHigh),
			Action:   string(policy.ActionBlock),
			Scope:    appcontrol.Scope{RouteKey: "missing"},
		}},
	})
	if err == nil || err.Error() != `route "missing" not found` {
		t.Fatalf("CreatePolicyBundle() error = %v, want missing route error", err)
	}

	err = store.CreatePolicyBundle(context.Background(), appcontrol.PolicyBundle{
		Key:           "pii",
		Version:       "1",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Rules: []appcontrol.Rule{{
			Key:      "provider-miss",
			Enabled:  true,
			Severity: string(policy.SeverityHigh),
			Action:   string(policy.ActionBlock),
			Scope:    appcontrol.Scope{Provider: "missing"},
		}},
	})
	if err == nil || err.Error() != `provider "missing" not found` {
		t.Fatalf("CreatePolicyBundle() error = %v, want missing provider error", err)
	}
}

func TestPostgresPolicyStoreConfigStatusReturnsCompileError(t *testing.T) {
	store := NewPolicyStore(&fakeConfigRepository{
		activeConfig: configSnapshot{
			PolicyBundles: []policystore.Bundle{{
				Key:           "broken",
				Version:       "1",
				Source:        string(policy.SourceUser),
				DefaultAction: string(policy.ActionAllow),
				Enabled:       true,
				Detectors: []policystore.Detector{{
					Key:     "broken-detector",
					Kind:    "unknown",
					Enabled: true,
				}},
			}},
		},
	})

	_, err := store.ConfigStatus(context.Background())
	if err == nil {
		t.Fatal("ConfigStatus() error = nil, want compile error")
	}
}

func postgresBundleFixture() policystore.Bundle {
	return policystore.Bundle{
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
			Scopes:       []policystore.RuleScope{{RouteID: "route-id", ProviderID: "provider-id", Model: "gpt-test", Direction: string(detection.DirectionInput)}},
		}},
	}
}

type fakeConfigRepository struct {
	loadActiveErr            error
	activeConfig             configSnapshot
	activation               policystore.ActivationResult
	activatedKeys            []string
	activationReason         string
	activationHash           string
	routes                   []routingstore.Route
	providers                []routingstore.Provider
	bundles                  []policystore.Bundle
	upsertedBundle           policystore.Bundle
	mappings                 []routingstore.ModelMapping
	upsertedMapping          routingstore.ModelMapping
	verdictProviders         []routingstore.VerdictProvider
	upsertedVerdictProvider  routingstore.VerdictProvider
	gatewayClients           []clientstore.GatewayClient
	upsertedClient           clientstore.GatewayClient
	revokedClientID          string
	routeLimits              []limitstore.RoutePolicy
	upsertedRouteLimit       limitstore.RoutePolicy
	clientRouteLimits        []limitstore.ClientRouteOverride
	upsertedClientRouteLimit limitstore.ClientRouteOverride
	deletedClientID          string
	deletedRouteID           string
}

func (r fakeConfigRepository) LoadActiveConfigSnapshot(ctx context.Context) (configSnapshot, error) {
	if r.loadActiveErr != nil {
		return configSnapshot{}, r.loadActiveErr
	}
	return r.activeConfig, nil
}

func (r fakeConfigRepository) ListRoutes(ctx context.Context) ([]routingstore.Route, error) {
	if r.loadActiveErr != nil {
		return nil, r.loadActiveErr
	}
	return append([]routingstore.Route(nil), r.routes...), nil
}

func (r fakeConfigRepository) ListProviders(ctx context.Context) ([]routingstore.Provider, error) {
	if r.loadActiveErr != nil {
		return nil, r.loadActiveErr
	}
	return append([]routingstore.Provider(nil), r.providers...), nil
}

func (r fakeConfigRepository) ListPolicyBundles(ctx context.Context) ([]policystore.Bundle, error) {
	if r.loadActiveErr != nil {
		return nil, r.loadActiveErr
	}
	return append([]policystore.Bundle(nil), r.bundles...), nil
}

func (r *fakeConfigRepository) UpsertPolicyBundle(ctx context.Context, bundle policystore.Bundle) error {
	r.upsertedBundle = bundle
	return nil
}

func (r *fakeConfigRepository) ActivatePolicyBundles(ctx context.Context, request policystore.ActivationRequest) (policystore.ActivationResult, error) {
	if r.loadActiveErr != nil && !errors.Is(r.loadActiveErr, configstore.ErrActiveConfigNotFound) {
		return policystore.ActivationResult{}, r.loadActiveErr
	}
	r.activatedKeys = append([]string(nil), request.Keys...)
	r.activationReason = request.Reason
	r.activationHash = request.SnapshotHash
	return r.activation, nil
}

func (r fakeConfigRepository) ListModelMappings(ctx context.Context) ([]routingstore.ModelMapping, error) {
	if r.loadActiveErr != nil {
		return nil, r.loadActiveErr
	}
	return append([]routingstore.ModelMapping(nil), r.mappings...), nil
}

func (r *fakeConfigRepository) UpsertModelMapping(ctx context.Context, mapping routingstore.ModelMapping) error {
	r.upsertedMapping = mapping
	return nil
}

func (r fakeConfigRepository) ListVerdictProviders(ctx context.Context) ([]routingstore.VerdictProvider, error) {
	if r.loadActiveErr != nil {
		return nil, r.loadActiveErr
	}
	return append([]routingstore.VerdictProvider(nil), r.verdictProviders...), nil
}

func (r *fakeConfigRepository) UpsertVerdictProvider(ctx context.Context, provider routingstore.VerdictProvider) error {
	r.upsertedVerdictProvider = provider
	return nil
}

func (r fakeConfigRepository) ListGatewayClients(ctx context.Context) ([]clientstore.GatewayClient, error) {
	return append([]clientstore.GatewayClient(nil), r.gatewayClients...), nil
}

func (r *fakeConfigRepository) CreateGatewayClient(ctx context.Context, client clientstore.GatewayClient) error {
	for _, existing := range r.gatewayClients {
		if existing.ExternalID == client.ExternalID || existing.KeyPrefix == client.KeyPrefix {
			return configstore.ErrGatewayClientAlreadyExists
		}
	}
	if client.ID == "" {
		client.ID = "postgres-" + client.ExternalID
	}
	r.gatewayClients = append(r.gatewayClients, client)
	r.upsertedClient = client
	return nil
}

func (r *fakeConfigRepository) UpsertGatewayClient(ctx context.Context, client clientstore.GatewayClient) error {
	replaced := false
	for i, existing := range r.gatewayClients {
		if existing.ExternalID == client.ExternalID {
			if client.ID == "" {
				client.ID = existing.ID
			}
			if existing.Status == "revoked" {
				client.Status = existing.Status
			}
			r.gatewayClients[i] = client
			replaced = true
			break
		}
	}
	if !replaced {
		if client.ID == "" {
			client.ID = "postgres-" + client.ExternalID
		}
		r.gatewayClients = append(r.gatewayClients, client)
	}
	r.upsertedClient = client
	return nil
}

func (r *fakeConfigRepository) RevokeGatewayClient(ctx context.Context, clientID string) error {
	r.revokedClientID = clientID
	for i := range r.gatewayClients {
		if r.gatewayClients[i].ExternalID == clientID || r.gatewayClients[i].ID == clientID {
			r.gatewayClients[i].Status = "revoked"
		}
	}
	return nil
}

func (r fakeConfigRepository) ListRouteLimitPolicies(ctx context.Context) ([]limitstore.RoutePolicy, error) {
	return append([]limitstore.RoutePolicy(nil), r.routeLimits...), nil
}

func (r *fakeConfigRepository) UpsertRouteLimitPolicy(ctx context.Context, policy limitstore.RoutePolicy) error {
	r.upsertedRouteLimit = policy
	return nil
}

func (r fakeConfigRepository) ListClientRouteLimitOverrides(ctx context.Context, clientID string) ([]limitstore.ClientRouteOverride, error) {
	items := make([]limitstore.ClientRouteOverride, 0, len(r.clientRouteLimits))
	for _, override := range r.clientRouteLimits {
		if clientID == "" || override.ClientID == clientID {
			items = append(items, override)
		}
	}
	return items, nil
}

func (r *fakeConfigRepository) UpsertClientRouteLimitOverride(ctx context.Context, override limitstore.ClientRouteOverride) error {
	r.upsertedClientRouteLimit = override
	return nil
}

func (r *fakeConfigRepository) DeleteClientRouteLimitOverride(ctx context.Context, clientID, routeID string) error {
	r.deletedClientID = clientID
	r.deletedRouteID = routeID
	return nil
}
