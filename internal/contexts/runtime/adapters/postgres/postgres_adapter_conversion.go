package runtime

import routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"

// convertPostgresRuntimeConfig converts the persisted runtime aggregate into the runtime contract.
func convertPostgresRuntimeConfig(input postgresRuntimeConfig) (RuntimeConfig, error) {
	lookups := newPostgresRuntimeLookups(input)

	routes := convertPostgresRoutes(input.Routes, lookups.modelMappingsByID)
	providers, err := convertPostgresProviders(input.Providers)
	if err != nil {
		return RuntimeConfig{}, err
	}
	modelMappings, err := convertPostgresModelMappings(input.ModelMappings, lookups.providerNames)
	if err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		Revision:                     RuntimeRevision{Number: input.Revision.Number},
		Routes:                       routes,
		Providers:                    providers,
		ModelMappings:                modelMappings,
		VerdictProviders:             convertPostgresVerdictProviders(input.VerdictProviders),
		RouteVerdictProviderBindings: convertPostgresRouteVerdictProviderBindings(input.RouteVerdictProviderBindings, lookups.routeNames, lookups.verdictProviderNames),
		Sinks:                        convertPostgresSinks(input.Sinks),
		Retention:                    convertPostgresRetentionPolicies(input.Retention, lookups.sinkNames),
		PolicyBundles:                convertPostgresPolicyBundles(input.PolicyBundles, lookups.routeNames, lookups.providerNames),
		RawCapture:                   convertPostgresRawCapturePolicies(input.RawCapture, lookups.routeNames),
		GatewayClients:               convertPostgresGatewayClients(input.GatewayClients),
		RouteLimits:                  convertPostgresRouteLimits(input.RouteLimitPolicies, lookups.routeNames),
		ClientRouteLimitOverrides:    convertPostgresClientRouteLimitOverrides(input.ClientRouteLimitOverrides, lookups.clientExternalIDs, lookups.routeNames),
	}, nil
}

// postgresRuntimeLookups caches identifier-to-name maps used during aggregate conversion.
type postgresRuntimeLookups struct {
	routeNames           map[string]string
	clientExternalIDs    map[string]string
	providerNames        map[string]string
	verdictProviderNames map[string]string
	sinkNames            map[string]string
	modelMappingsByID    map[string]routingstore.ModelMapping
}

// newPostgresRuntimeLookups builds identifier lookup tables for runtime conversion.
func newPostgresRuntimeLookups(input postgresRuntimeConfig) postgresRuntimeLookups {
	routeNames := make(map[string]string, len(input.Routes))
	for _, route := range input.Routes {
		routeNames[route.ID] = route.Name
	}

	clientExternalIDs := make(map[string]string, len(input.GatewayClients))
	for _, client := range input.GatewayClients {
		clientExternalIDs[client.ID] = client.ExternalID
	}

	providerNames := make(map[string]string, len(input.Providers))
	for _, provider := range input.Providers {
		providerNames[provider.ID] = provider.Name
	}

	verdictProviderNames := make(map[string]string, len(input.VerdictProviders))
	for _, provider := range input.VerdictProviders {
		verdictProviderNames[provider.ID] = provider.Name
	}

	sinkNames := make(map[string]string, len(input.Sinks))
	for _, sink := range input.Sinks {
		sinkNames[sink.ID] = sink.Name
	}

	mappings := make(map[string]routingstore.ModelMapping, len(input.ModelMappings))
	for _, mapping := range input.ModelMappings {
		mappings[mapping.ID] = mapping
	}

	return postgresRuntimeLookups{
		routeNames:           routeNames,
		clientExternalIDs:    clientExternalIDs,
		providerNames:        providerNames,
		verdictProviderNames: verdictProviderNames,
		sinkNames:            sinkNames,
		modelMappingsByID:    mappings,
	}
}
