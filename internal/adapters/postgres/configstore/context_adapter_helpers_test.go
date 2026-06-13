package configstore

import (
	"context"
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

type testRuntimeGraph struct {
	Revision                     revisionstore.ConfigRevision
	Routes                       []routingstore.Route
	Providers                    []routingstore.Provider
	ModelMappings                []routingstore.ModelMapping
	VerdictProviders             []routingstore.VerdictProvider
	RouteVerdictProviderBindings []routingstore.RouteVerdictProviderBinding
	PolicyBundles                []policystore.Bundle
	Sinks                        []observabilitystore.Sink
	Retention                    []observabilitystore.RetentionPolicy
	RawCapture                   []observabilitystore.RawCapturePolicy
	GatewayClients               []clientstore.GatewayClient
	RouteLimitPolicies           []limitstore.RoutePolicy
	ClientRouteLimitOverrides    []limitstore.ClientRouteOverride
}

func testLoadActiveRuntimeConfig(ctx context.Context, repo *ConfigRepository) (testRuntimeGraph, error) {
	var cfg testRuntimeGraph
	err := repo.WithActiveRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revision revisionstore.ConfigRevision) error {
		var err error
		cfg = testRuntimeGraph{Revision: revision}
		if cfg.Routes, err = routingstore.LoadRoutes(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.Providers, err = routingstore.LoadProviders(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.ModelMappings, err = routingstore.LoadModelMappings(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.VerdictProviders, err = routingstore.LoadVerdictProviders(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.RouteVerdictProviderBindings, err = routingstore.LoadRouteVerdictProviderBindings(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.PolicyBundles, err = policystore.LoadBundles(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.Sinks, err = observabilitystore.LoadSinks(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.Retention, err = observabilitystore.LoadRetentionPolicies(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.RawCapture, err = observabilitystore.LoadRawCapturePolicies(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.GatewayClients, err = clientstore.LoadGatewayClients(ctx, q); err != nil {
			return err
		}
		if cfg.RouteLimitPolicies, err = limitstore.LoadRoutePolicies(ctx, q, revision.ID); err != nil {
			return err
		}
		cfg.ClientRouteLimitOverrides, err = limitstore.LoadClientRouteOverrides(ctx, q, revision.ID, "")
		return err
	})
	return cfg, err
}

func testListRoutes(ctx context.Context, repo *ConfigRepository) ([]routingstore.Route, error) {
	var routes []routingstore.Route
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		routes, err = routingstore.LoadRoutes(ctx, q, revisionID)
		return err
	})
	return routes, err
}

func testListProviders(ctx context.Context, repo *ConfigRepository) ([]routingstore.Provider, error) {
	var providers []routingstore.Provider
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		providers, err = routingstore.LoadProviders(ctx, q, revisionID)
		return err
	})
	return providers, err
}

func testListModelMappings(ctx context.Context, repo *ConfigRepository) ([]routingstore.ModelMapping, error) {
	var mappings []routingstore.ModelMapping
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		mappings, err = routingstore.LoadModelMappings(ctx, q, revisionID)
		return err
	})
	return mappings, err
}

func testUpsertModelMapping(ctx context.Context, repo *ConfigRepository, mapping routingstore.ModelMapping) error {
	return repo.WithDraftRevision(ctx, "upsert model mapping", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return routingstore.UpsertModelMapping(ctx, q, revisionID, mapping)
	})
}

func testListVerdictProviders(ctx context.Context, repo *ConfigRepository) ([]routingstore.VerdictProvider, error) {
	var providers []routingstore.VerdictProvider
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		providers, err = routingstore.LoadVerdictProviders(ctx, q, revisionID)
		return err
	})
	return providers, err
}

func testUpsertVerdictProvider(ctx context.Context, repo *ConfigRepository, provider routingstore.VerdictProvider) error {
	return repo.WithDraftRevision(ctx, "upsert verdict provider", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return routingstore.UpsertVerdictProvider(ctx, q, revisionID, provider)
	})
}

func testListPolicyBundles(ctx context.Context, repo *ConfigRepository) ([]policystore.Bundle, error) {
	var bundles []policystore.Bundle
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		bundles, err = policystore.LoadBundles(ctx, q, revisionID)
		return err
	})
	return bundles, err
}

func testUpsertPolicyBundle(ctx context.Context, repo *ConfigRepository, bundle policystore.Bundle) error {
	return repo.WithDraftRevision(ctx, "upsert policy bundle", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return policystore.UpsertBundle(ctx, q, revisionID, bundle)
	})
}

func testActivatePolicyBundles(ctx context.Context, repo *ConfigRepository, request policystore.ActivationRequest) (policystore.ActivationResult, error) {
	result, err := repo.ActivateDraftRevision(ctx, revisionstore.ActivationRequest{
		Actor:        request.Actor,
		Reason:       request.Reason,
		SnapshotHash: request.SnapshotHash,
	}, revisionstore.ActivationHooks{
		ValidateDraft: func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
			bundles, err := policystore.LoadBundlesByKeys(ctx, q, revisionID, request.Keys)
			if err != nil {
				return err
			}
			if len(bundles) != len(request.Keys) {
				return fmt.Errorf("activate policy bundles: requested bundle not found")
			}
			return nil
		},
		BeforeActivation: func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
			return policystore.UpdateDraftBundleActivation(ctx, q, revisionID, request.Keys)
		},
		RecordActivation: func(ctx context.Context, q revisionstore.Queryer, record revisionstore.ActivationRecord) error {
			return policystore.RecordActivation(ctx, q, record.RevisionID, record.SnapshotID, record.Actor, record.Reason)
		},
	})
	if err != nil {
		return policystore.ActivationResult{}, err
	}
	return policystore.ActivationResult{
		RevisionNumber: result.RevisionNumber,
		SnapshotHash:   result.SnapshotHash,
		ActiveKeys:     append([]string(nil), request.Keys...),
	}, nil
}

func testListGatewayClients(ctx context.Context, repo *ConfigRepository) ([]clientstore.GatewayClient, error) {
	var clients []clientstore.GatewayClient
	err := repo.WithTransaction(ctx, "list gateway clients", func(ctx context.Context, q revisionstore.Queryer) error {
		var err error
		clients, err = clientstore.LoadGatewayClients(ctx, q)
		return err
	})
	return clients, err
}

func testCreateGatewayClient(ctx context.Context, repo *ConfigRepository, client clientstore.GatewayClient) error {
	return repo.WithTransaction(ctx, "create gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		if err := clientstore.Create(ctx, q, client); err != nil {
			return err
		}
		return clientstore.BumpGeneration(ctx, q, ConfigActivatedChannel)
	})
}

func testUpsertGatewayClient(ctx context.Context, repo *ConfigRepository, client clientstore.GatewayClient) error {
	return repo.WithTransaction(ctx, "upsert gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		if err := clientstore.Upsert(ctx, q, client); err != nil {
			return err
		}
		return clientstore.BumpGeneration(ctx, q, ConfigActivatedChannel)
	})
}

func testRevokeGatewayClient(ctx context.Context, repo *ConfigRepository, clientID string) error {
	return repo.WithTransaction(ctx, "revoke gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		changed, err := clientstore.Revoke(ctx, q, clientID)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return clientstore.BumpGeneration(ctx, q, ConfigActivatedChannel)
	})
}

func testListRouteLimitPolicies(ctx context.Context, repo *ConfigRepository) ([]limitstore.RoutePolicy, error) {
	var policies []limitstore.RoutePolicy
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		policies, err = limitstore.LoadRoutePolicies(ctx, q, revisionID)
		return err
	})
	return policies, err
}

func testUpsertRouteLimitPolicy(ctx context.Context, repo *ConfigRepository, policy limitstore.RoutePolicy) error {
	return repo.WithDraftRevision(ctx, "upsert route limit policy", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		routeID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, policy.RouteID)
		if err != nil {
			return err
		}
		policy.RouteID = routeID
		return limitstore.UpsertRoutePolicy(ctx, q, revisionID, policy)
	})
}

func testListClientRouteLimitOverrides(ctx context.Context, repo *ConfigRepository, clientID string) ([]limitstore.ClientRouteOverride, error) {
	var overrides []limitstore.ClientRouteOverride
	err := repo.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		overrides, err = limitstore.LoadClientRouteOverrides(ctx, q, revisionID, clientID)
		return err
	})
	return overrides, err
}

func testUpsertClientRouteLimitOverride(ctx context.Context, repo *ConfigRepository, override limitstore.ClientRouteOverride) error {
	return repo.WithDraftRevision(ctx, "upsert client route limit override", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		routeID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, override.RouteID)
		if err != nil {
			return err
		}
		override.RouteID = routeID
		return limitstore.UpsertClientRouteOverride(ctx, q, revisionID, override)
	})
}

func testDeleteClientRouteLimitOverride(ctx context.Context, repo *ConfigRepository, clientID, routeID string) error {
	return repo.WithDraftRevision(ctx, "delete client route limit override", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		currentRouteID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, routeID)
		if err != nil {
			return err
		}
		return limitstore.DeleteClientRouteOverride(ctx, q, revisionID, clientID, currentRouteID)
	})
}
