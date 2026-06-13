package revisionstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// activeRevision reads the active revision row, including metadata needed by orchestration layers.
func activeRevision(ctx context.Context, q Queryer) (ConfigRevision, error) {
	var revision ConfigRevision
	err := q.QueryRow(ctx, `
		select cr.id::text, cr.revision_number + coalesce(gv.generation, 0), cr.source, cr.status, cr.actor,
			cr.compiled_snapshot_hash, cr.compiled_snapshot_ref, cr.activated_at
		from config_revisions cr
		cross join gateway_client_config_versions gv
		where cr.status = 'active'
		order by cr.revision_number desc
		limit 1
	`).Scan(
		&revision.ID,
		&revision.Number,
		&revision.Source,
		&revision.Status,
		&revision.Actor,
		&revision.CompiledSnapshotHash,
		&revision.CompiledSnapshotRef,
		&revision.ActivatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ConfigRevision{}, ErrActiveRevisionNotFound
	}
	if err != nil {
		return ConfigRevision{}, fmt.Errorf("load active config revision: %w", err)
	}
	return revision, nil
}

// activeRevisionNumber reads the active revision token after applying gateway-client generation offsets.
func activeRevisionNumber(ctx context.Context, q Queryer) (int64, error) {
	var number int64
	err := q.QueryRow(ctx, `
		select cr.revision_number + coalesce(gv.generation, 0)
		from config_revisions cr
		cross join gateway_client_config_versions gv
		where cr.status = 'active'
		order by cr.revision_number desc
		limit 1
	`).Scan(&number)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrActiveRevisionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("load active revision number: %w", err)
	}
	return number, nil
}

// latestDraftRevisionID reads the newest draft revision identifier when draft state exists.
func latestDraftRevisionID(ctx context.Context, q Queryer) (string, error) {
	var id string
	err := q.QueryRow(ctx, `
		select id::text
		from config_revisions
		where status = 'draft'
		order by revision_number desc
		limit 1
	`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrActiveRevisionNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load draft config revision: %w", err)
	}
	return id, nil
}
