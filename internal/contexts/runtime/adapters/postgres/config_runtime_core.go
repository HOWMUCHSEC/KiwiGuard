package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
)

// configRuntimeCore provides runtime-owned access to shared revision orchestration primitives.
type configRuntimeCore struct {
	core revisionUnitOfWork
}

// ActiveRevisionNumber delegates revision-token reads to the shared revision orchestration core.
func (r configRuntimeCore) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.core.ActiveRevisionNumber(ctx)
}

// LoadRuntimeConfig hydrates the active runtime aggregate through runtime-owned PostgreSQL loading.
func (r configRuntimeCore) LoadRuntimeConfig(ctx context.Context) (postgresRuntimeConfig, error) {
	return loadPostgresRuntimeConfig(ctx, r)
}

// WithActiveRevision runs fn inside a read-only transaction scoped to the active revision.
func (r configRuntimeCore) WithActiveRevision(ctx context.Context, fn func(context.Context, revisionstore.Queryer, revisionstore.ConfigRevision) error) error {
	return r.core.WithActiveRevision(ctx, fn)
}

// newConfigRuntimeCore builds the runtime adapter's shared revision orchestration core.
func newConfigRuntimeCore(pool revisionstore.DB) configRuntimeCore {
	return configRuntimeCore{core: configstore.NewRevisionUnitOfWork(pool)}
}
