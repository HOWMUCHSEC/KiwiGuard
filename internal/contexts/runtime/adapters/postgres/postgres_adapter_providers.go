package runtime

import (
	"encoding/json"
	"fmt"

	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// convertPostgresProviders converts persisted upstream providers into runtime configuration.
func convertPostgresProviders(input []routingstore.Provider) ([]ProviderConfig, error) {
	providers := make([]ProviderConfig, 0, len(input))
	for _, provider := range input {
		headers, err := decodeStringMap(provider.Headers)
		if err != nil {
			return nil, fmt.Errorf("decode provider %s headers: %w", provider.Name, err)
		}
		providers = append(providers, ProviderConfig{
			Key:           provider.Name,
			BaseURL:       provider.BaseURL,
			CredentialRef: provider.CredentialRef,
			Headers:       headers,
			Timeout:       provider.Timeout,
		})
	}
	return providers, nil
}

// convertPostgresVerdictProviders converts persisted verdict providers into runtime configuration.
func convertPostgresVerdictProviders(input []routingstore.VerdictProvider) []VerdictProviderConfig {
	verdictProviders := make([]VerdictProviderConfig, 0, len(input))
	for _, provider := range input {
		verdictProviders = append(verdictProviders, VerdictProviderConfig{
			Key:           provider.Name,
			Name:          provider.Name,
			Endpoint:      provider.Endpoint,
			CredentialRef: provider.CredentialRef,
			Enabled:       provider.Enabled,
			Timeout:       provider.Timeout,
		})
	}
	return verdictProviders
}

// convertPostgresRouteVerdictProviderBindings converts persisted route bindings into runtime configuration.
func convertPostgresRouteVerdictProviderBindings(bindings []routingstore.RouteVerdictProviderBinding, routeNames map[string]string, verdictProviderNames map[string]string) []RouteVerdictProviderBindingConfig {
	converted := make([]RouteVerdictProviderBindingConfig, 0, len(bindings))
	for _, binding := range bindings {
		converted = append(converted, RouteVerdictProviderBindingConfig{
			RouteKey:           routeNames[binding.RouteID],
			VerdictProviderKey: verdictProviderNames[binding.VerdictProviderID],
			ExecutionMode:      binding.ExecutionMode,
			Disabled:           !binding.Enabled,
			Priority:           binding.Priority,
		})
	}
	return converted
}

func decodeStringMap(raw json.RawMessage) (map[string]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}
