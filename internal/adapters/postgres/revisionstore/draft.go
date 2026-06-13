package revisionstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ensureDraftRevision reuses the latest draft when present or materializes one from the active revision graph.
func (u *UnitOfWork) ensureDraftRevision(ctx context.Context, q Queryer) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		select id::text
		from config_revisions
		where status = 'draft'
		order by revision_number desc
		limit 1
	`).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("load draft config revision: %w", err)
	}

	var activeID string
	err = q.QueryRow(ctx, `
		select id::text
		from config_revisions
		where status = 'active'
		order by revision_number desc
		limit 1
	`).Scan(&activeID)
	if err == nil {
		if u.cloneActiveRevision == nil {
			return "", fmt.Errorf("clone active config revision: clone hook is required")
		}
		draftID, err := createEmptyDraftRevision(ctx, q)
		if err != nil {
			return "", err
		}
		if err := u.cloneActiveRevision(ctx, q, activeID, draftID); err != nil {
			return "", err
		}
		return draftID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("load active config revision for draft clone: %w", err)
	}

	return createEmptyDraftRevision(ctx, q)
}

// createEmptyDraftRevision inserts a new draft revision row with control-plane defaults.
func createEmptyDraftRevision(ctx context.Context, q Queryer) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		insert into config_revisions (source, status, summary, actor, validation_status)
		values ('control', 'draft', 'control changes', 'system', 'pending')
		returning id::text
	`).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create draft config revision: %w", err)
	}
	return id, nil
}
