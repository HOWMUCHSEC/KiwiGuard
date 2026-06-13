package runtime

import (
	"time"

	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
)

// convertPostgresGatewayClients converts persisted gateway clients into runtime configuration.
func convertPostgresGatewayClients(clients []clientstore.GatewayClient) []GatewayClientConfig {
	gatewayClients := make([]GatewayClientConfig, 0, len(clients))
	for _, client := range clients {
		gatewayClients = append(gatewayClients, GatewayClientConfig{
			ID:        client.ExternalID,
			Name:      client.Name,
			Status:    client.Status,
			KeyPrefix: client.KeyPrefix,
			KeyHash:   client.KeyHash,
		})
	}
	return gatewayClients
}

// convertPostgresRouteLimits converts persisted route limit policies into runtime configuration.
func convertPostgresRouteLimits(policies []limitstore.RoutePolicy, routeNames map[string]string) []RouteLimitConfig {
	routeLimits := make([]RouteLimitConfig, 0, len(policies))
	for _, policy := range policies {
		routeLimits = append(routeLimits, RouteLimitConfig{
			RouteKey:              routeNames[policy.RouteID],
			RequestsPerWindow:     policy.RequestsPerWindow,
			Window:                time.Duration(policy.WindowSeconds) * time.Second,
			MaxConcurrentRequests: policy.MaxConcurrentRequests,
			MaxBodyBytes:          policy.MaxBodyBytes,
			Disabled:              !policy.Enabled,
		})
	}
	return routeLimits
}

// convertPostgresClientRouteLimitOverrides converts persisted client overrides into runtime configuration.
func convertPostgresClientRouteLimitOverrides(overrides []limitstore.ClientRouteOverride, clientExternalIDs map[string]string, routeNames map[string]string) []ClientRouteLimitOverrideConfig {
	clientRouteLimitOverrides := make([]ClientRouteLimitOverrideConfig, 0, len(overrides))
	for _, override := range overrides {
		clientRouteLimitOverrides = append(clientRouteLimitOverrides, ClientRouteLimitOverrideConfig{
			ClientID:              clientExternalIDs[override.ClientID],
			RouteKey:              routeNames[override.RouteID],
			RequestsPerWindow:     override.RequestsPerWindow,
			Window:                time.Duration(override.WindowSeconds) * time.Second,
			MaxConcurrentRequests: override.MaxConcurrentRequests,
			MaxBodyBytes:          override.MaxBodyBytes,
			Disabled:              !override.Enabled,
		})
	}
	return clientRouteLimitOverrides
}
