package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
)

// loadPostgresRuntimeConfig hydrates the full active runtime aggregate inside one active-revision scope.
func loadPostgresRuntimeConfig(ctx context.Context, repo activeRevisionRepository) (postgresRuntimeConfig, error) {
	var cfg postgresRuntimeConfig
	err := repo.WithActiveRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revision revisionstore.ConfigRevision) error {
		cfg = postgresRuntimeConfig{Revision: revision}
		if err := loadRuntimeRouting(ctx, q, revision.ID, &cfg); err != nil {
			return err
		}
		if err := loadRuntimePolicy(ctx, q, revision.ID, &cfg); err != nil {
			return err
		}
		if err := loadRuntimeObservability(ctx, q, revision.ID, &cfg); err != nil {
			return err
		}
		return loadRuntimeClientsAndLimits(ctx, q, revision.ID, &cfg)
	})
	return cfg, err
}
