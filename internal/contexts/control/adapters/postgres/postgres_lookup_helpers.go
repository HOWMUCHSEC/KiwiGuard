package postgres

import (
	"fmt"

	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// routeNamesByID indexes route names by their persisted identifiers.
func routeNamesByID(routes []routingstore.Route) map[string]string {
	names := make(map[string]string, len(routes))
	for _, route := range routes {
		names[route.ID] = route.Name
	}
	return names
}

// providerNamesByID indexes provider names by their persisted identifiers.
func providerNamesByID(providers []routingstore.Provider) map[string]string {
	names := make(map[string]string, len(providers))
	for _, provider := range providers {
		names[provider.ID] = provider.Name
	}
	return names
}

// routeIDsByName indexes route identifiers by route key.
func routeIDsByName(routes []routingstore.Route) map[string]string {
	ids := make(map[string]string, len(routes))
	for _, route := range routes {
		ids[route.Name] = route.ID
	}
	return ids
}

// providerIDsByName indexes provider identifiers by provider key.
func providerIDsByName(providers []routingstore.Provider) map[string]string {
	ids := make(map[string]string, len(providers))
	for _, provider := range providers {
		ids[provider.Name] = provider.ID
	}
	return ids
}

// clientIDsByExternalID indexes persisted client identifiers by external client ID.
func clientIDsByExternalID(clients []clientstore.GatewayClient) map[string]string {
	ids := make(map[string]string, len(clients))
	for _, client := range clients {
		ids[client.ExternalID] = client.ID
	}
	return ids
}

// clientExternalIDsByID indexes external client IDs by persisted identifier.
func clientExternalIDsByID(clients []clientstore.GatewayClient) map[string]string {
	ids := make(map[string]string, len(clients))
	for _, client := range clients {
		ids[client.ID] = client.ExternalID
	}
	return ids
}

// requiredClientIDByExternalID resolves an external client ID or returns the standard not-found error.
func requiredClientIDByExternalID(externalID string, clients []clientstore.GatewayClient) (string, error) {
	id, ok := clientIDsByExternalID(clients)[externalID]
	if !ok {
		return "", errGatewayClientNotFound
	}
	return id, nil
}

// optionalIDByName resolves an optional named dependency into its persisted identifier.
func optionalIDByName(name string, ids map[string]string, kind string) (string, error) {
	if name == "" {
		return "", nil
	}
	id, ok := ids[name]
	if !ok {
		return "", fmt.Errorf("%s %q not found", kind, name)
	}
	return id, nil
}

// requiredIDByName resolves a required named dependency into its persisted identifier.
func requiredIDByName(name string, ids map[string]string, kind string) (string, error) {
	id, ok := ids[name]
	if !ok {
		return "", fmt.Errorf("%s %q not found", kind, name)
	}
	return id, nil
}

// firstNonEmpty returns the first non-empty string from values.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
