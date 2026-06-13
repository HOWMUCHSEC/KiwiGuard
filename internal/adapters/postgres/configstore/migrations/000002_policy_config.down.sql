drop table if exists policy_activation_records;
drop table if exists compiled_snapshots;
drop table if exists raw_capture_policies;
drop table if exists retention_policies;
drop table if exists sinks;
drop table if exists route_verdict_provider_bindings;
drop table if exists route_policy_bindings;
drop table if exists policy_rule_scopes;
drop table if exists policy_rule_detectors;
drop table if exists policy_rules;
drop table if exists policy_detectors;
drop table if exists policy_bundles;

alter table routes
    drop constraint if exists routes_model_mapping_id_fkey;

drop table if exists model_mappings;

alter table verdict_providers
    drop column if exists max_concurrency,
    drop column if exists circuit_breaker_config,
    drop column if exists retry_config,
    drop column if exists model_name,
    drop column if exists adapter_config,
    drop column if exists credential_ref;

alter table providers
    drop column if exists capabilities,
    drop column if exists headers,
    drop column if exists circuit_breaker_config,
    drop column if exists retry_config,
    drop column if exists provider_type;

alter table routes
    drop column if exists model_mapping_id,
    drop column if exists path,
    drop column if exists method,
    drop column if exists priority,
    drop column if exists enabled;

alter table config_revisions
    drop column if exists compiled_snapshot_ref,
    drop column if exists compiled_snapshot_hash,
    drop column if exists validation_errors,
    drop column if exists validation_status,
    drop column if exists actor;
