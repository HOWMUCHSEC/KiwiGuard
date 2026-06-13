package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// loadRuntimeRouting hydrates every routing-side aggregate needed for one runtime revision.
func loadRuntimeRouting(ctx context.Context, q revisionstore.Queryer, revisionID string, cfg *postgresRuntimeConfig) error {
	var err error
	if cfg.Routes, err = routingstore.LoadRoutes(ctx, q, revisionID); err != nil {
		return err
	}
	if cfg.Providers, err = routingstore.LoadProviders(ctx, q, revisionID); err != nil {
		return err
	}
	if cfg.ModelMappings, err = routingstore.LoadModelMappings(ctx, q, revisionID); err != nil {
		return err
	}
	if cfg.VerdictProviders, err = routingstore.LoadVerdictProviders(ctx, q, revisionID); err != nil {
		return err
	}
	cfg.RouteVerdictProviderBindings, err = routingstore.LoadRouteVerdictProviderBindings(ctx, q, revisionID)
	return err
}
