package revisionstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ActivationRequest describes generic activation metadata for a draft revision.
type ActivationRequest struct {
	Actor        string
	Reason       string
	SnapshotHash string
}

// ActivationHooks let owning contexts validate and record activation details.
type ActivationHooks struct {
	ValidateDraft    func(context.Context, Queryer, string) error
	BeforeActivation func(context.Context, Queryer, string) error
	RecordActivation func(context.Context, Queryer, ActivationRecord) error
}

// ActivationRecord identifies the revision and compiled snapshot activated by core orchestration.
type ActivationRecord struct {
	RevisionID string
	SnapshotID string
	Actor      string
	Reason     string
}

// RevisionActivationResult reports the revision token and snapshot hash produced by activation.
type RevisionActivationResult struct {
	RevisionNumber int64
	SnapshotHash   string
}

// ActivateDraftRevision promotes the current draft revision after context-owned validation hooks run.
func (u *UnitOfWork) ActivateDraftRevision(ctx context.Context, request ActivationRequest, hooks ActivationHooks) (RevisionActivationResult, error) {
	tx, err := u.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return RevisionActivationResult{}, fmt.Errorf("begin activate draft revision transaction: %w", err)
	}
	defer rollback(ctx, tx)

	revisionID, err := latestDraftRevisionID(ctx, tx)
	if err != nil {
		return RevisionActivationResult{}, err
	}
	if hooks.ValidateDraft != nil {
		if err := hooks.ValidateDraft(ctx, tx, revisionID); err != nil {
			return RevisionActivationResult{}, err
		}
	}
	if request.SnapshotHash == "" {
		return RevisionActivationResult{}, fmt.Errorf("activate draft revision: snapshot hash is required")
	}
	if hooks.BeforeActivation != nil {
		if err := hooks.BeforeActivation(ctx, tx, revisionID); err != nil {
			return RevisionActivationResult{}, err
		}
	}

	var snapshotID string
	err = tx.QueryRow(ctx, `
		insert into compiled_snapshots (revision_id, snapshot_hash, status, compiled_at)
		values ($1, $2, 'compiled', now())
		on conflict (revision_id, snapshot_hash) do update
		set status = excluded.status,
			compiled_at = excluded.compiled_at,
			error_details = '[]'::jsonb
		returning id
	`, revisionID, request.SnapshotHash).Scan(&snapshotID)
	if err != nil {
		return RevisionActivationResult{}, fmt.Errorf("record compiled snapshot: %w", err)
	}
	if _, err := tx.Exec(ctx, `update config_revisions set status = 'rejected' where status = 'active'`); err != nil {
		return RevisionActivationResult{}, fmt.Errorf("deactivate previous config revisions: %w", err)
	}

	actor := request.Actor
	if actor == "" {
		actor = "system"
	}
	if _, err := tx.Exec(ctx, `
		update config_revisions
		set status = 'active',
			actor = $2,
			validation_status = 'valid',
			validation_errors = '[]'::jsonb,
			compiled_snapshot_hash = $3,
			activated_at = now()
		where id = $1
	`, revisionID, actor, request.SnapshotHash); err != nil {
		return RevisionActivationResult{}, fmt.Errorf("activate config revision: %w", err)
	}
	if hooks.RecordActivation != nil {
		if err := hooks.RecordActivation(ctx, tx, ActivationRecord{
			RevisionID: revisionID,
			SnapshotID: snapshotID,
			Actor:      actor,
			Reason:     request.Reason,
		}); err != nil {
			return RevisionActivationResult{}, err
		}
	}
	runtimeRevisionNumber, err := activeRevisionNumber(ctx, tx)
	if err != nil {
		return RevisionActivationResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return RevisionActivationResult{}, fmt.Errorf("commit activate draft revision transaction: %w", err)
	}
	return RevisionActivationResult{RevisionNumber: runtimeRevisionNumber, SnapshotHash: request.SnapshotHash}, nil
}
