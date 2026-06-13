package configstore

import (
	"context"

	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

// cloneSinks copies sink configuration into the target draft revision.
func cloneSinks(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	return observabilitystore.CloneSinks(ctx, q, sourceRevisionID, draftRevisionID)
}

// cloneRetentionPolicies copies retention settings into the target draft revision.
func cloneRetentionPolicies(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string, sinkIDs map[string]string) error {
	return observabilitystore.CloneRetentionPolicies(ctx, q, sourceRevisionID, draftRevisionID, sinkIDs)
}

// cloneRawCapturePolicies copies raw-capture settings into the target draft revision.
func cloneRawCapturePolicies(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	return observabilitystore.CloneRawCapturePolicies(ctx, q, sourceRevisionID, draftRevisionID, routeIDs)
}
