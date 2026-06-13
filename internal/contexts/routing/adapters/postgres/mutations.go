package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UpsertModelMapping writes a model mapping into a draft revision.
func UpsertModelMapping(ctx context.Context, q Queryer, revisionID string, mapping ModelMapping) error {
	if mapping.Parameters == nil {
		mapping.Parameters = json.RawMessage(`{}`)
	}
	_, err := q.Exec(ctx, `
		insert into model_mappings (revision_id, name, source_model, target_provider_id, target_model, parameters)
		values ($1, $2, $3, nullif($4, '')::uuid, $5, $6)
		on conflict (revision_id, name) do update
		set source_model = excluded.source_model,
			target_provider_id = excluded.target_provider_id,
			target_model = excluded.target_model,
			parameters = excluded.parameters,
			updated_at = now()
	`, revisionID, mapping.Name, mapping.SourceModel, mapping.TargetProviderID, mapping.TargetModel, mapping.Parameters)
	if err != nil {
		return fmt.Errorf("upsert model mapping: %w", err)
	}
	return nil
}

// UpsertVerdictProvider writes a verdict provider into a draft revision.
func UpsertVerdictProvider(ctx context.Context, q Queryer, revisionID string, provider VerdictProvider) error {
	if provider.Adapter == "" {
		provider.Adapter = "http"
	}
	if provider.Timeout <= 0 {
		provider.Timeout = 5 * time.Second
	}
	if provider.MaxConcurrency <= 0 {
		provider.MaxConcurrency = 16
	}
	provider.AdapterConfig = defaultJSONObject(provider.AdapterConfig)
	provider.RetryConfig = defaultJSONObject(provider.RetryConfig)
	provider.CircuitBreakerConfig = defaultJSONObject(provider.CircuitBreakerConfig)

	_, err := q.Exec(ctx, `
		insert into verdict_providers (revision_id, name, adapter, endpoint, timeout_ms, credential_ref,
			adapter_config, model_name, retry_config, circuit_breaker_config, max_concurrency, enabled)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		on conflict (revision_id, name) do update
		set adapter = excluded.adapter,
			endpoint = excluded.endpoint,
			timeout_ms = excluded.timeout_ms,
			credential_ref = excluded.credential_ref,
			adapter_config = excluded.adapter_config,
			model_name = excluded.model_name,
			retry_config = excluded.retry_config,
			circuit_breaker_config = excluded.circuit_breaker_config,
			max_concurrency = excluded.max_concurrency,
			enabled = excluded.enabled
	`, revisionID, provider.Name, provider.Adapter, provider.Endpoint, durationMillis(provider.Timeout), provider.CredentialRef, provider.AdapterConfig, provider.ModelName, provider.RetryConfig, provider.CircuitBreakerConfig, provider.MaxConcurrency, provider.Enabled)
	if err != nil {
		return fmt.Errorf("upsert verdict provider: %w", err)
	}
	return nil
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	return raw
}

func durationMillis(d time.Duration) int {
	return int(d / time.Millisecond)
}
