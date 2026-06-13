package configstore

import "context"

// cloneActiveRevisionAsDraft copies all context-owned config records into a draft revision.
func cloneActiveRevisionAsDraft(ctx context.Context, q queryer, activeRevisionID, draftID string) error {
	providerIDs, err := cloneProviders(ctx, q, activeRevisionID, draftID)
	if err != nil {
		return err
	}
	mappingIDs, err := cloneModelMappings(ctx, q, activeRevisionID, draftID, providerIDs)
	if err != nil {
		return err
	}
	routeIDs, err := cloneRoutes(ctx, q, activeRevisionID, draftID, mappingIDs)
	if err != nil {
		return err
	}
	if err := cloneGatewayLimitRecords(ctx, q, activeRevisionID, draftID, routeIDs); err != nil {
		return err
	}
	verdictProviderIDs, err := cloneVerdictProviders(ctx, q, activeRevisionID, draftID)
	if err != nil {
		return err
	}
	bundleIDs, detectorIDs, ruleIDs, err := clonePolicyGraph(ctx, q, activeRevisionID, draftID)
	if err != nil {
		return err
	}
	if err := clonePolicyBindings(ctx, q, activeRevisionID, ruleIDs, detectorIDs, routeIDs, providerIDs, bundleIDs, verdictProviderIDs); err != nil {
		return err
	}
	sinkIDs, err := cloneSinks(ctx, q, activeRevisionID, draftID)
	if err != nil {
		return err
	}
	if err := cloneRetentionPolicies(ctx, q, activeRevisionID, draftID, sinkIDs); err != nil {
		return err
	}
	if err := cloneRawCapturePolicies(ctx, q, activeRevisionID, draftID, routeIDs); err != nil {
		return err
	}

	return nil
}
