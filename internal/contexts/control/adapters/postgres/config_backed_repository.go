package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
)

// configBackedRepository adapts revision-scoped config storage to control-plane persistence ports.
type configBackedRepository struct {
	core revisionUnitOfWork
}

// revisionUnitOfWork captures the shared revision orchestration primitives needed by control storage.
type revisionUnitOfWork interface {
	WithCurrentRevision(context.Context, func(context.Context, revisionstore.Queryer, string) error) error
	WithActiveRevision(context.Context, func(context.Context, revisionstore.Queryer, revisionstore.ConfigRevision) error) error
	WithDraftRevision(context.Context, string, func(context.Context, revisionstore.Queryer, string) error) error
	WithTransaction(context.Context, string, func(context.Context, revisionstore.Queryer) error) error
	ActivateDraftRevision(context.Context, revisionstore.ActivationRequest, revisionstore.ActivationHooks) (revisionstore.RevisionActivationResult, error)
}

// newConfigBackedRepository builds a control-plane repository on top of the shared revision core.
func newConfigBackedRepository(core revisionUnitOfWork) *configBackedRepository {
	return &configBackedRepository{core: core}
}
