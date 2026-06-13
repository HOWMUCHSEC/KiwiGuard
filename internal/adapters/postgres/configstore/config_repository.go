package configstore

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrActiveConfigNotFound is returned when no active runtime config exists.
	ErrActiveConfigNotFound = revisionstore.ErrActiveRevisionNotFound
	// ErrGatewayClientAlreadyExists is returned when creating a duplicate gateway client.
	ErrGatewayClientAlreadyExists = clientstore.ErrAlreadyExists
)

// ConfigRepository exposes shared PostgreSQL configuration persistence primitives.
type ConfigRepository struct {
	pool         configDB
	revisionCore *revisionstore.UnitOfWork
}

// NewConfigRepository builds the shared PostgreSQL config repository facade.
func NewConfigRepository(pool *pgxpool.Pool) *ConfigRepository {
	return &ConfigRepository{pool: pool, revisionCore: newRevisionUnitOfWork(pool)}
}

// NewRevisionUnitOfWork creates the shared revision transaction core.
func NewRevisionUnitOfWork(db revisionstore.DB) *revisionstore.UnitOfWork {
	return newRevisionUnitOfWork(db)
}

type configDB interface {
	revisionstore.DB
}

type queryer = revisionstore.Queryer

func newRevisionUnitOfWork(db revisionstore.DB) *revisionstore.UnitOfWork {
	return revisionstore.NewUnitOfWork(db, revisionstore.Options{
		CloneActiveRevision: cloneActiveRevisionAsDraft,
	})
}

func (r *ConfigRepository) revisions() *revisionstore.UnitOfWork {
	if r.revisionCore != nil {
		return r.revisionCore
	}
	return newRevisionUnitOfWork(r.pool)
}

// WithCurrentRevision runs fn against the current draft revision when present,
// otherwise against the active revision.
func (r *ConfigRepository) WithCurrentRevision(ctx context.Context, fn func(context.Context, revisionstore.Queryer, string) error) error {
	return r.revisions().WithCurrentRevision(ctx, fn)
}

// WithActiveRevision runs fn inside a repeatable-read read-only transaction
// against the active revision.
func (r *ConfigRepository) WithActiveRevision(ctx context.Context, fn func(context.Context, revisionstore.Queryer, revisionstore.ConfigRevision) error) error {
	return r.revisions().WithActiveRevision(ctx, fn)
}

// WithDraftRevision runs fn inside a transaction against an existing or cloned draft revision.
func (r *ConfigRepository) WithDraftRevision(ctx context.Context, label string, fn func(context.Context, revisionstore.Queryer, string) error) error {
	return r.revisions().WithDraftRevision(ctx, label, fn)
}

// WithTransaction runs fn inside a generic PostgreSQL transaction.
func (r *ConfigRepository) WithTransaction(ctx context.Context, label string, fn func(context.Context, revisionstore.Queryer) error) error {
	return r.revisions().WithTransaction(ctx, label, fn)
}

// ActivateDraftRevision promotes the current draft revision after context-owned validation hooks run.
func (r *ConfigRepository) ActivateDraftRevision(ctx context.Context, request revisionstore.ActivationRequest, hooks revisionstore.ActivationHooks) (revisionstore.RevisionActivationResult, error) {
	return r.revisions().ActivateDraftRevision(ctx, request, hooks)
}

// ActiveRevisionNumber reads the active revision token tracked by the shared config repository.
func (r *ConfigRepository) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	return r.revisions().ActiveRevisionNumber(ctx)
}
