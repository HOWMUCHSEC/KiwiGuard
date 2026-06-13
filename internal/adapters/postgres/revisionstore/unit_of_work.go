package revisionstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrActiveRevisionNotFound is returned when no active runtime revision exists.
var ErrActiveRevisionNotFound = errors.New("active runtime config not found")

// Queryer is the PostgreSQL contract exposed to context-owned persistence adapters.
type Queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// DB is the PostgreSQL contract required by the revision unit of work.
type DB interface {
	Queryer
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// CloneActiveRevisionFunc clones an active revision graph into an already-created draft revision.
type CloneActiveRevisionFunc func(context.Context, Queryer, string, string) error

// UnitOfWork coordinates revision-scoped PostgreSQL transactions.
type UnitOfWork struct {
	db                  DB
	cloneActiveRevision CloneActiveRevisionFunc
}

// Options customizes revision unit-of-work behavior.
type Options struct {
	CloneActiveRevision CloneActiveRevisionFunc
}

// NewUnitOfWork builds the shared PostgreSQL transaction boundary for config revisions.
func NewUnitOfWork(db DB, options Options) *UnitOfWork {
	return &UnitOfWork{
		db:                  db,
		cloneActiveRevision: options.CloneActiveRevision,
	}
}

// CurrentRevisionID selects the mutable draft revision when it exists, otherwise the active revision.
func (u *UnitOfWork) CurrentRevisionID(ctx context.Context) (string, error) {
	var id string
	err := u.db.QueryRow(ctx, `
		select id
		from config_revisions
		where status in ('draft', 'active')
		order by case status when 'draft' then 0 else 1 end, revision_number desc
		limit 1
	`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrActiveRevisionNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load current config revision: %w", err)
	}
	return id, nil
}

// ActiveRevisionNumber reads the active revision token exposed to runtime reload decisions.
func (u *UnitOfWork) ActiveRevisionNumber(ctx context.Context) (int64, error) {
	number, err := activeRevisionNumber(ctx, u.db)
	if err != nil {
		return 0, err
	}
	return number, nil
}

// WithCurrentRevision runs fn against the current draft revision when present,
// otherwise against the active revision.
func (u *UnitOfWork) WithCurrentRevision(ctx context.Context, fn func(context.Context, Queryer, string) error) error {
	revisionID, err := u.CurrentRevisionID(ctx)
	if err != nil {
		return err
	}
	return fn(ctx, u.db, revisionID)
}

// WithActiveRevision runs fn inside a repeatable-read read-only transaction
// against the active revision.
func (u *UnitOfWork) WithActiveRevision(ctx context.Context, fn func(context.Context, Queryer, ConfigRevision) error) error {
	tx, err := u.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("begin active revision transaction: %w", err)
	}
	defer rollback(ctx, tx)

	revision, err := activeRevision(ctx, tx)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx, revision); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit active revision transaction: %w", err)
	}
	return nil
}

// WithDraftRevision runs fn inside a transaction against an existing or cloned draft revision.
func (u *UnitOfWork) WithDraftRevision(ctx context.Context, label string, fn func(context.Context, Queryer, string) error) error {
	tx, err := u.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin %s transaction: %w", label, err)
	}
	defer rollback(ctx, tx)

	revisionID, err := u.ensureDraftRevision(ctx, tx)
	if err != nil {
		return err
	}
	if err := fn(ctx, tx, revisionID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s transaction: %w", label, err)
	}
	return nil
}

// WithTransaction runs fn inside a generic PostgreSQL transaction.
func (u *UnitOfWork) WithTransaction(ctx context.Context, label string, fn func(context.Context, Queryer) error) error {
	tx, err := u.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin %s transaction: %w", label, err)
	}
	defer rollback(ctx, tx)

	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit %s transaction: %w", label, err)
	}
	return nil
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}
