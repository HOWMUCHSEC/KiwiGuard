package runtime

import (
	"encoding/json"
	"fmt"

	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// postgresModelMappingParams captures transport-specific metadata stored on mapping rows.
type postgresModelMappingParams struct {
	RouteKey string `json:"route_key"`
	Provider string `json:"provider"`
	Enabled  *bool  `json:"enabled"`
}

// convertPostgresRoutes converts persisted routes into runtime route configuration.
func convertPostgresRoutes(input []routingstore.Route, mappings map[string]routingstore.ModelMapping) []RouteConfig {
	routes := make([]RouteConfig, 0, len(input))
	for _, route := range input {
		mapping := mappings[route.ModelMappingID]
		routes = append(routes, RouteConfig{
			Key:            route.Name,
			Method:         route.Method,
			Path:           firstNonEmpty(route.Path, route.PathPrefix),
			ProviderKey:    route.Provider,
			RequestedModel: mapping.SourceModel,
			MappedModel:    mapping.TargetModel,
			UpstreamModel:  firstNonEmpty(mapping.TargetModel, route.UpstreamModel),
			ExecutionMode:  route.ExecutionMode,
			FallbackAction: route.FallbackAction,
			Disabled:       !route.Enabled,
		})
	}
	return routes
}

// convertPostgresModelMappings converts persisted model mappings into runtime configuration.
func convertPostgresModelMappings(input []routingstore.ModelMapping, providerNames map[string]string) ([]ModelMappingConfig, error) {
	modelMappings := make([]ModelMappingConfig, 0, len(input))
	for _, mapping := range input {
		params, err := decodeModelMappingParams(mapping.Parameters)
		if err != nil {
			return nil, fmt.Errorf("decode model mapping %s parameters: %w", mapping.Name, err)
		}
		providerKey := firstNonEmpty(params.Provider, providerNames[mapping.TargetProviderID])
		modelMappings = append(modelMappings, ModelMappingConfig{
			Key:            mapping.Name,
			RouteKey:       params.RouteKey,
			ProviderKey:    providerKey,
			RequestedModel: mapping.SourceModel,
			MappedModel:    mapping.TargetModel,
			UpstreamModel:  mapping.TargetModel,
			Disabled:       params.Enabled != nil && !*params.Enabled,
		})
	}
	return modelMappings, nil
}

// decodeModelMappingParams decodes optional transport metadata stored on mapping rows.
func decodeModelMappingParams(raw json.RawMessage) (postgresModelMappingParams, error) {
	if len(raw) == 0 {
		return postgresModelMappingParams{}, nil
	}
	var params postgresModelMappingParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return postgresModelMappingParams{}, err
	}
	return params, nil
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
