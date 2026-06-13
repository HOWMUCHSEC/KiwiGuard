package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// ListRoutes reads route records from the revision currently visible to control APIs.
func (r *configBackedRepository) ListRoutes(ctx context.Context) ([]routingstore.Route, error) {
	var routes []routingstore.Route
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		routes, err = routingstore.LoadRoutes(ctx, q, revisionID)
		return err
	})
	return routes, err
}

func (r *configBackedRepository) ListProviders(ctx context.Context) ([]routingstore.Provider, error) {
	var providers []routingstore.Provider
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		providers, err = routingstore.LoadProviders(ctx, q, revisionID)
		return err
	})
	return providers, err
}

func (r *configBackedRepository) ListModelMappings(ctx context.Context) ([]routingstore.ModelMapping, error) {
	var mappings []routingstore.ModelMapping
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		mappings, err = routingstore.LoadModelMappings(ctx, q, revisionID)
		return err
	})
	return mappings, err
}

func (r *configBackedRepository) UpsertModelMapping(ctx context.Context, mapping routingstore.ModelMapping) error {
	return r.core.WithDraftRevision(ctx, "upsert model mapping", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return routingstore.UpsertModelMapping(ctx, q, revisionID, mapping)
	})
}

func (r *configBackedRepository) ListVerdictProviders(ctx context.Context) ([]routingstore.VerdictProvider, error) {
	var providers []routingstore.VerdictProvider
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		providers, err = routingstore.LoadVerdictProviders(ctx, q, revisionID)
		return err
	})
	return providers, err
}

func (r *configBackedRepository) UpsertVerdictProvider(ctx context.Context, provider routingstore.VerdictProvider) error {
	return r.core.WithDraftRevision(ctx, "upsert verdict provider", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return routingstore.UpsertVerdictProvider(ctx, q, revisionID, provider)
	})
}
