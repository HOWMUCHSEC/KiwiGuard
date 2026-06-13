package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
)

// ListRouteLimitPolicies reads the default route-limit policies visible from the current config revision.
func (r *configBackedRepository) ListRouteLimitPolicies(ctx context.Context) ([]limitstore.RoutePolicy, error) {
	var policies []limitstore.RoutePolicy
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		policies, err = limitstore.LoadRoutePolicies(ctx, q, revisionID)
		return err
	})
	return policies, err
}

func (r *configBackedRepository) UpsertRouteLimitPolicy(ctx context.Context, policy limitstore.RoutePolicy) error {
	return r.core.WithDraftRevision(ctx, "upsert route limit policy", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		routeID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, policy.RouteID)
		if err != nil {
			return err
		}
		policy.RouteID = routeID
		return limitstore.UpsertRoutePolicy(ctx, q, revisionID, policy)
	})
}

func (r *configBackedRepository) ListClientRouteLimitOverrides(ctx context.Context, clientID string) ([]limitstore.ClientRouteOverride, error) {
	var overrides []limitstore.ClientRouteOverride
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		overrides, err = limitstore.LoadClientRouteOverrides(ctx, q, revisionID, clientID)
		return err
	})
	return overrides, err
}

func (r *configBackedRepository) UpsertClientRouteLimitOverride(ctx context.Context, override limitstore.ClientRouteOverride) error {
	return r.core.WithDraftRevision(ctx, "upsert client route limit override", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		routeID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, override.RouteID)
		if err != nil {
			return err
		}
		override.RouteID = routeID
		return limitstore.UpsertClientRouteOverride(ctx, q, revisionID, override)
	})
}

func (r *configBackedRepository) DeleteClientRouteLimitOverride(ctx context.Context, clientID, routeID string) error {
	return r.core.WithDraftRevision(ctx, "delete client route limit override", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		currentRouteID, err := limitstore.RouteIDForRevision(ctx, q, revisionID, routeID)
		if err != nil {
			return err
		}
		return limitstore.DeleteClientRouteOverride(ctx, q, revisionID, clientID, currentRouteID)
	})
}
