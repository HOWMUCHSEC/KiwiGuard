package postgres

import (
	"context"
	"fmt"
	"time"
)

// LoadRoutes hydrates the route records attached to one config revision.
func LoadRoutes(ctx context.Context, q Queryer, revisionID string) ([]Route, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, enabled, priority, method, path, path_prefix, upstream_provider,
			upstream_model, coalesce(model_mapping_id::text, ''), execution_mode, fallback_action
		from routes
		where revision_id = $1
		order by priority, name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load routes: %w", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		var route Route
		if err := rows.Scan(&route.ID, &route.Name, &route.Enabled, &route.Priority, &route.Method, &route.Path, &route.PathPrefix, &route.Provider, &route.UpstreamModel, &route.ModelMappingID, &route.ExecutionMode, &route.FallbackAction); err != nil {
			return nil, fmt.Errorf("scan route: %w", err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routes: %w", err)
	}
	return routes, nil
}

// LoadProviders hydrates the upstream provider records attached to one config revision.
func LoadProviders(ctx context.Context, q Queryer, revisionID string) ([]Provider, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, base_url, credential_ref, timeout_ms, provider_type,
			headers, retry_config, circuit_breaker_config, capabilities
		from providers
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load providers: %w", err)
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		var provider Provider
		var timeoutMS int
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.BaseURL, &provider.CredentialRef, &timeoutMS, &provider.ProviderType, &provider.Headers, &provider.RetryConfig, &provider.CircuitBreakerConfig, &provider.Capabilities); err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}
		provider.Timeout = time.Duration(timeoutMS) * time.Millisecond
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers: %w", err)
	}
	return providers, nil
}

// LoadModelMappings hydrates the model-mapping records attached to one config revision.
func LoadModelMappings(ctx context.Context, q Queryer, revisionID string) ([]ModelMapping, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, source_model, coalesce(target_provider_id::text, ''), target_model, parameters
		from model_mappings
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load model mappings: %w", err)
	}
	defer rows.Close()

	var mappings []ModelMapping
	for rows.Next() {
		var mapping ModelMapping
		if err := rows.Scan(&mapping.ID, &mapping.Name, &mapping.SourceModel, &mapping.TargetProviderID, &mapping.TargetModel, &mapping.Parameters); err != nil {
			return nil, fmt.Errorf("scan model mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model mappings: %w", err)
	}
	return mappings, nil
}

// LoadVerdictProviders hydrates the verdict-provider records attached to one config revision.
func LoadVerdictProviders(ctx context.Context, q Queryer, revisionID string) ([]VerdictProvider, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, adapter, endpoint, timeout_ms, credential_ref, adapter_config,
			model_name, retry_config, circuit_breaker_config, max_concurrency, enabled
		from verdict_providers
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load verdict providers: %w", err)
	}
	defer rows.Close()

	var providers []VerdictProvider
	for rows.Next() {
		var provider VerdictProvider
		var timeoutMS int
		if err := rows.Scan(&provider.ID, &provider.Name, &provider.Adapter, &provider.Endpoint, &timeoutMS, &provider.CredentialRef, &provider.AdapterConfig, &provider.ModelName, &provider.RetryConfig, &provider.CircuitBreakerConfig, &provider.MaxConcurrency, &provider.Enabled); err != nil {
			return nil, fmt.Errorf("scan verdict provider: %w", err)
		}
		provider.Timeout = time.Duration(timeoutMS) * time.Millisecond
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verdict providers: %w", err)
	}
	return providers, nil
}

// LoadRouteVerdictProviderBindings hydrates route-to-verdict bindings attached to one config revision.
func LoadRouteVerdictProviderBindings(ctx context.Context, q Queryer, revisionID string) ([]RouteVerdictProviderBinding, error) {
	rows, err := q.Query(ctx, `
		select b.id::text, b.route_id::text, b.verdict_provider_id::text,
			b.enabled, b.execution_mode, b.priority
		from route_verdict_provider_bindings b
		join routes r on r.id = b.route_id
		where r.revision_id = $1
		order by r.name, b.priority, b.id
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load route verdict provider bindings: %w", err)
	}
	defer rows.Close()

	var bindings []RouteVerdictProviderBinding
	for rows.Next() {
		var binding RouteVerdictProviderBinding
		if err := rows.Scan(&binding.ID, &binding.RouteID, &binding.VerdictProviderID, &binding.Enabled, &binding.ExecutionMode, &binding.Priority); err != nil {
			return nil, fmt.Errorf("scan route verdict provider binding: %w", err)
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate route verdict provider bindings: %w", err)
	}
	return bindings, nil
}
