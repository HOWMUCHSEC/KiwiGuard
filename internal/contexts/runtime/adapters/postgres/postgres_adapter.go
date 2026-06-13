package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	appruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/application"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository bridges PostgreSQL-backed config loading into the runtime application repository port.
type Repository struct {
	repo postgresRuntimeConfigRepository
}

type postgresRuntimeConfigRepository interface {
	ActiveRevisionNumber(context.Context) (int64, error)
	LoadRuntimeConfig(context.Context) (postgresRuntimeConfig, error)
}

type activeRevisionRepository interface {
	WithActiveRevision(context.Context, func(context.Context, revisionstore.Queryer, revisionstore.ConfigRevision) error) error
}

type revisionUnitOfWork interface {
	ActiveRevisionNumber(context.Context) (int64, error)
	WithActiveRevision(context.Context, func(context.Context, revisionstore.Queryer, revisionstore.ConfigRevision) error) error
}

// NewRepository wraps PostgreSQL runtime loading behind the runtime repository port.
func NewRepository(repo postgresRuntimeConfigRepository) *Repository {
	return &Repository{repo: repo}
}

// NewRepositoryFromPool builds the runtime PostgreSQL adapter from a shared pool.
func NewRepositoryFromPool(pool *pgxpool.Pool) *Repository {
	return NewRepository(newConfigRuntimeCore(pool))
}

// ActiveRevisionNumber reads the active revision token used to decide whether runtime state is stale.
func (r *Repository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	if r.repo == nil {
		return 0, fmt.Errorf("load active revision number: repository is required")
	}
	revision, err := r.repo.ActiveRevisionNumber(ctx)
	if err != nil {
		return 0, mapPostgresRuntimeError(err)
	}
	return revision, nil
}

// LoadRuntimeConfig hydrates the active PostgreSQL aggregate and converts it into the runtime contract.
func (r *Repository) LoadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	if r.repo == nil {
		return RuntimeConfig{}, fmt.Errorf("load runtime config: repository is required")
	}
	cfg, err := r.repo.LoadRuntimeConfig(ctx)
	if err != nil {
		return RuntimeConfig{}, mapPostgresRuntimeError(err)
	}
	converted, err := convertPostgresRuntimeConfig(cfg)
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("convert postgres runtime config: %w", err)
	}
	return converted, nil
}

func mapPostgresRuntimeError(err error) error {
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		return appruntime.ErrActiveRuntimeConfigNotFound
	}
	return err
}
