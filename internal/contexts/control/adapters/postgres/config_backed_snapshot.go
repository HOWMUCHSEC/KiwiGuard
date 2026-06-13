package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// LoadActiveConfigSnapshot hydrates the smallest active aggregate needed by control status endpoints.
func (r *configBackedRepository) LoadActiveConfigSnapshot(ctx context.Context) (configSnapshot, error) {
	var cfg configSnapshot
	err := r.core.WithActiveRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revision revisionstore.ConfigRevision) error {
		var err error
		if cfg.Routes, err = routingstore.LoadRoutes(ctx, q, revision.ID); err != nil {
			return err
		}
		if cfg.Providers, err = routingstore.LoadProviders(ctx, q, revision.ID); err != nil {
			return err
		}
		cfg.PolicyBundles, err = policystore.LoadBundles(ctx, q, revision.ID)
		return err
	})
	return cfg, err
}
