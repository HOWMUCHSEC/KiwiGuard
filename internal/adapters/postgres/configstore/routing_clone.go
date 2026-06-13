package configstore

import (
	"context"

	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// cloneProviders copies upstream providers into the target draft revision.
func cloneProviders(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	return routingstore.CloneProviders(ctx, q, sourceRevisionID, draftRevisionID)
}

// cloneModelMappings copies model mappings into the target draft revision.
func cloneModelMappings(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string, providerIDs map[string]string) (map[string]string, error) {
	return routingstore.CloneModelMappings(ctx, q, sourceRevisionID, draftRevisionID, providerIDs)
}

// cloneRoutes copies routes into the target draft revision.
func cloneRoutes(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string, mappingIDs map[string]string) (map[string]string, error) {
	return routingstore.CloneRoutes(ctx, q, sourceRevisionID, draftRevisionID, mappingIDs)
}

// cloneVerdictProviders copies verdict providers into the target draft revision.
func cloneVerdictProviders(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	return routingstore.CloneVerdictProviders(ctx, q, sourceRevisionID, draftRevisionID)
}

// cloneRouteVerdictProviderBindings copies route-to-verdict-provider bindings into the draft.
func cloneRouteVerdictProviderBindings(ctx context.Context, q queryer, sourceRevisionID string, routeIDs, verdictProviderIDs map[string]string) error {
	return routingstore.CloneRouteVerdictProviderBindings(ctx, q, sourceRevisionID, routeIDs, verdictProviderIDs)
}

// cloneGatewayLimitRecords copies route and client limit records into the target draft revision.
func cloneGatewayLimitRecords(ctx context.Context, q queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	return limitstore.CloneGatewayRecords(ctx, q, sourceRevisionID, draftRevisionID, routeIDs)
}
