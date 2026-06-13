package configstore

import (
	"context"

	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
)

// clonePolicyGraph copies policy bundles, detectors, and rules into the target draft revision.
func clonePolicyGraph(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string) (map[string]string, map[string]string, map[string]string, error) {
	return policystore.CloneGraph(ctx, q, sourceRevisionID, draftRevisionID)
}

// clonePolicyBindings copies policy-owned bindings after core policy records have been cloned.
func clonePolicyBindings(ctx context.Context, q queryer, sourceRevisionID string, ruleIDs, detectorIDs, routeIDs, providerIDs, bundleIDs, verdictProviderIDs map[string]string) error {
	if err := policystore.CloneBindings(ctx, q, sourceRevisionID, ruleIDs, detectorIDs, routeIDs, providerIDs, bundleIDs); err != nil {
		return err
	}
	return cloneRouteVerdictProviderBindings(ctx, q, sourceRevisionID, routeIDs, verdictProviderIDs)
}
