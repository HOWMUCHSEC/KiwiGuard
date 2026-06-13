package postgres

import (
	"context"
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
)

// ListPolicyBundles reads policy bundles from the revision currently visible to control APIs.
func (r *configBackedRepository) ListPolicyBundles(ctx context.Context) ([]policystore.Bundle, error) {
	var bundles []policystore.Bundle
	err := r.core.WithCurrentRevision(ctx, func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		var err error
		bundles, err = policystore.LoadBundles(ctx, q, revisionID)
		return err
	})
	return bundles, err
}

func (r *configBackedRepository) UpsertPolicyBundle(ctx context.Context, bundle policystore.Bundle) error {
	return r.core.WithDraftRevision(ctx, "upsert policy bundle", func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
		return policystore.UpsertBundle(ctx, q, revisionID, bundle)
	})
}

func (r *configBackedRepository) ActivatePolicyBundles(ctx context.Context, request policystore.ActivationRequest) (policystore.ActivationResult, error) {
	result, err := r.core.ActivateDraftRevision(ctx, revisionstore.ActivationRequest{
		Actor:        request.Actor,
		Reason:       request.Reason,
		SnapshotHash: request.SnapshotHash,
	}, revisionstore.ActivationHooks{
		ValidateDraft: func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
			bundles, err := policystore.LoadBundlesByKeys(ctx, q, revisionID, request.Keys)
			if err != nil {
				return err
			}
			if len(bundles) != len(request.Keys) {
				return fmt.Errorf("activate policy bundles: requested bundle not found")
			}
			return nil
		},
		BeforeActivation: func(ctx context.Context, q revisionstore.Queryer, revisionID string) error {
			return policystore.UpdateDraftBundleActivation(ctx, q, revisionID, request.Keys)
		},
		RecordActivation: func(ctx context.Context, q revisionstore.Queryer, record revisionstore.ActivationRecord) error {
			return policystore.RecordActivation(ctx, q, record.RevisionID, record.SnapshotID, record.Actor, record.Reason)
		},
	})
	if err != nil {
		return policystore.ActivationResult{}, err
	}
	return policystore.ActivationResult{
		RevisionNumber: result.RevisionNumber,
		SnapshotHash:   result.SnapshotHash,
		ActiveKeys:     append([]string(nil), request.Keys...),
	}, nil
}
