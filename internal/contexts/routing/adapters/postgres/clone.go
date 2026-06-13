package postgres

import (
	"context"
	"fmt"
)

// CloneProviders clones providers from an active revision into a draft revision.
func CloneProviders(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	type providerClone struct {
		oldID     string
		provider  Provider
		timeoutMS int
	}
	rows, err := q.Query(ctx, `
		select id::text, name, base_url, credential_ref, timeout_ms, provider_type,
			headers, retry_config, circuit_breaker_config, capabilities
		from providers
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load providers for draft clone: %w", err)
	}
	defer rows.Close()

	var items []providerClone
	for rows.Next() {
		var item providerClone
		if err := rows.Scan(&item.oldID, &item.provider.Name, &item.provider.BaseURL, &item.provider.CredentialRef, &item.timeoutMS, &item.provider.ProviderType, &item.provider.Headers, &item.provider.RetryConfig, &item.provider.CircuitBreakerConfig, &item.provider.Capabilities); err != nil {
			return nil, fmt.Errorf("scan provider for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate providers for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		err := q.QueryRow(ctx, `
			insert into providers (revision_id, name, base_url, credential_ref, timeout_ms, provider_type,
				headers, retry_config, circuit_breaker_config, capabilities)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			returning id::text
		`, draftRevisionID, item.provider.Name, item.provider.BaseURL, item.provider.CredentialRef, item.timeoutMS, item.provider.ProviderType, item.provider.Headers, item.provider.RetryConfig, item.provider.CircuitBreakerConfig, item.provider.Capabilities).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone provider %s: %w", item.provider.Name, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneModelMappings clones model mappings from an active revision into a draft revision.
func CloneModelMappings(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, providerIDs map[string]string) (map[string]string, error) {
	type mappingClone struct {
		oldID         string
		oldProviderID string
		mapping       ModelMapping
	}
	rows, err := q.Query(ctx, `
		select id::text, name, source_model, coalesce(target_provider_id::text, ''), target_model, parameters
		from model_mappings
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load model mappings for draft clone: %w", err)
	}
	defer rows.Close()

	var items []mappingClone
	for rows.Next() {
		var item mappingClone
		if err := rows.Scan(&item.oldID, &item.mapping.Name, &item.mapping.SourceModel, &item.oldProviderID, &item.mapping.TargetModel, &item.mapping.Parameters); err != nil {
			return nil, fmt.Errorf("scan model mapping for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model mappings for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		targetProviderID, err := remapOptionalID(item.oldProviderID, providerIDs, "model mapping target provider")
		if err != nil {
			return nil, err
		}
		err = q.QueryRow(ctx, `
			insert into model_mappings (revision_id, name, source_model, target_provider_id, target_model, parameters)
			values ($1, $2, $3, nullif($4, '')::uuid, $5, $6)
			returning id::text
		`, draftRevisionID, item.mapping.Name, item.mapping.SourceModel, targetProviderID, item.mapping.TargetModel, item.mapping.Parameters).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone model mapping %s: %w", item.mapping.Name, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneRoutes clones routes from an active revision into a draft revision.
func CloneRoutes(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, mappingIDs map[string]string) (map[string]string, error) {
	type routeClone struct {
		oldID        string
		oldMappingID string
		route        Route
	}
	rows, err := q.Query(ctx, `
		select id::text, name, path_prefix, upstream_provider, upstream_model, execution_mode,
			fallback_action, enabled, priority, method, path, coalesce(model_mapping_id::text, '')
		from routes
		where revision_id = $1
		order by priority, name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load routes for draft clone: %w", err)
	}
	defer rows.Close()

	var items []routeClone
	for rows.Next() {
		var item routeClone
		if err := rows.Scan(&item.oldID, &item.route.Name, &item.route.PathPrefix, &item.route.Provider, &item.route.UpstreamModel, &item.route.ExecutionMode, &item.route.FallbackAction, &item.route.Enabled, &item.route.Priority, &item.route.Method, &item.route.Path, &item.oldMappingID); err != nil {
			return nil, fmt.Errorf("scan route for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate routes for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		modelMappingID, err := remapOptionalID(item.oldMappingID, mappingIDs, "route model mapping")
		if err != nil {
			return nil, err
		}
		err = q.QueryRow(ctx, `
			insert into routes (revision_id, name, path_prefix, upstream_provider, upstream_model,
				execution_mode, fallback_action, enabled, priority, method, path, model_mapping_id)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, nullif($12, '')::uuid)
			returning id::text
		`, draftRevisionID, item.route.Name, item.route.PathPrefix, item.route.Provider, item.route.UpstreamModel, item.route.ExecutionMode, item.route.FallbackAction, item.route.Enabled, item.route.Priority, item.route.Method, item.route.Path, modelMappingID).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone route %s: %w", item.route.Name, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneVerdictProviders clones verdict providers from an active revision into a draft revision.
func CloneVerdictProviders(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	type verdictProviderClone struct {
		oldID     string
		provider  VerdictProvider
		timeoutMS int
	}
	rows, err := q.Query(ctx, `
		select id::text, name, adapter, endpoint, timeout_ms, credential_ref, adapter_config,
			model_name, retry_config, circuit_breaker_config, max_concurrency, enabled
		from verdict_providers
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load verdict providers for draft clone: %w", err)
	}
	defer rows.Close()

	var items []verdictProviderClone
	for rows.Next() {
		var item verdictProviderClone
		if err := rows.Scan(&item.oldID, &item.provider.Name, &item.provider.Adapter, &item.provider.Endpoint, &item.timeoutMS, &item.provider.CredentialRef, &item.provider.AdapterConfig, &item.provider.ModelName, &item.provider.RetryConfig, &item.provider.CircuitBreakerConfig, &item.provider.MaxConcurrency, &item.provider.Enabled); err != nil {
			return nil, fmt.Errorf("scan verdict provider for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate verdict providers for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		err := q.QueryRow(ctx, `
			insert into verdict_providers (revision_id, name, adapter, endpoint, timeout_ms, credential_ref,
				adapter_config, model_name, retry_config, circuit_breaker_config, max_concurrency, enabled)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			returning id::text
		`, draftRevisionID, item.provider.Name, item.provider.Adapter, item.provider.Endpoint, item.timeoutMS, item.provider.CredentialRef, item.provider.AdapterConfig, item.provider.ModelName, item.provider.RetryConfig, item.provider.CircuitBreakerConfig, item.provider.MaxConcurrency, item.provider.Enabled).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone verdict provider %s: %w", item.provider.Name, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneRouteVerdictProviderBindings clones route verdict-provider bindings into a draft revision.
func CloneRouteVerdictProviderBindings(ctx context.Context, q Queryer, sourceRevisionID string, routeIDs, verdictProviderIDs map[string]string) error {
	type routeVerdictBindingClone struct {
		oldRouteID           string
		oldVerdictProviderID string
		enabled              bool
		executionMode        string
		priority             int
	}
	rows, err := q.Query(ctx, `
		select b.route_id::text, b.verdict_provider_id::text, b.enabled, b.execution_mode, b.priority
		from route_verdict_provider_bindings b
		join routes r on r.id = b.route_id
		where r.revision_id = $1
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load route verdict provider bindings for draft clone: %w", err)
	}
	defer rows.Close()

	var items []routeVerdictBindingClone
	for rows.Next() {
		var item routeVerdictBindingClone
		if err := rows.Scan(&item.oldRouteID, &item.oldVerdictProviderID, &item.enabled, &item.executionMode, &item.priority); err != nil {
			return fmt.Errorf("scan route verdict provider binding for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate route verdict provider bindings for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		routeID, err := remapRequiredID(item.oldRouteID, routeIDs, "route verdict provider binding route")
		if err != nil {
			return err
		}
		verdictProviderID, err := remapRequiredID(item.oldVerdictProviderID, verdictProviderIDs, "route verdict provider binding verdict provider")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into route_verdict_provider_bindings (route_id, verdict_provider_id, enabled, execution_mode, priority)
			values ($1, $2, $3, $4, $5)
		`, routeID, verdictProviderID, item.enabled, item.executionMode, item.priority); err != nil {
			return fmt.Errorf("clone route verdict provider binding: %w", err)
		}
	}
	return nil
}

func remapRequiredID(oldID string, ids map[string]string, label string) (string, error) {
	if newID, ok := ids[oldID]; ok {
		return newID, nil
	}
	return "", fmt.Errorf("missing cloned %s id for %s", label, oldID)
}

func remapOptionalID(oldID string, ids map[string]string, label string) (string, error) {
	if oldID == "" {
		return "", nil
	}
	return remapRequiredID(oldID, ids, label)
}
