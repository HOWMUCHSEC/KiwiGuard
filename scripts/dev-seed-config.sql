begin;

update config_revisions
set status = 'rejected'
where status in ('active', 'draft');

with revision as (
    insert into config_revisions (
        source,
        status,
        summary,
        actor,
        validation_status,
        validation_errors,
        compiled_snapshot_hash,
        compiled_snapshot_ref,
        activated_at
    )
    values (
        'dev_seed',
        'active',
        'local development seed',
        'dev-env',
        'valid',
        '[]'::jsonb,
        'dev-seed',
        'local',
        now()
    )
    returning id
),
provider as (
    insert into providers (
        revision_id,
        name,
        base_url,
        credential_ref,
        timeout_ms,
        provider_type,
        headers,
        retry_config,
        circuit_breaker_config,
        capabilities
    )
    select
        id,
        'dev-openai',
        :'dev_mock_llm_url',
        '',
        5000,
        'openai_compatible',
        '{}'::jsonb,
        '{}'::jsonb,
        '{}'::jsonb,
        '{"chat_completions":true,"responses":true}'::jsonb
    from revision
    returning id, revision_id
),
mapping as (
    insert into model_mappings (
        revision_id,
        name,
        source_model,
        target_provider_id,
        target_model,
        parameters
    )
    select
        revision_id,
        'dev-chat',
        'kiwiguard-dev',
        id,
        'mock-secure-model',
        '{"route_key":"chat-completions","provider":"dev-openai","enabled":true}'::jsonb
    from provider
    returning id, revision_id
),
route as (
    insert into routes (
        revision_id,
        name,
        enabled,
        priority,
        method,
        path,
        path_prefix,
        upstream_provider,
        upstream_model,
        model_mapping_id,
        execution_mode,
        fallback_action
    )
    select
        revision_id,
        'chat-completions',
        true,
        10,
        'POST',
        '/v1/chat/completions',
        '/v1/chat/completions',
        'dev-openai',
        'mock-secure-model',
        id,
        'inline',
        'block'
    from mapping
    returning id, revision_id
),
verdict_provider as (
    insert into verdict_providers (
        revision_id,
        name,
        adapter,
        endpoint,
        credential_ref,
        model_name,
        timeout_ms,
        adapter_config,
        retry_config,
        circuit_breaker_config,
        max_concurrency,
        enabled
    )
    select
        revision_id,
        'dev-security-model',
        'http',
        :'dev_mock_llm_url' || '/verdict',
        '',
        'mock-security-verdict',
        3000,
        '{}'::jsonb,
        '{}'::jsonb,
        '{}'::jsonb,
        16,
        true
    from route
    returning id, revision_id
),
bundle as (
    insert into policy_bundles (
        revision_id,
        name,
        source,
        version,
        description,
        enabled,
        metadata
    )
    select
        revision_id,
        'dev-observability',
        'user',
        '2026.05',
        'Local development regex rule for capture verification.',
        true,
        '{}'::jsonb
    from route
    returning id, revision_id
),
detector as (
    insert into policy_detectors (
        bundle_id,
        name,
        detector_type,
        pattern,
        config,
        enabled
    )
    select
        id,
        'email-regex',
        'regex',
        '[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}',
        '{"categories":["pii.email"]}'::jsonb,
        true
    from bundle
    returning id, bundle_id
),
rule as (
    insert into policy_rules (
        bundle_id,
        name,
        description,
        severity,
        action,
        enabled,
        priority,
        config
    )
    select
        bundle_id,
        'shadow-log-email',
        'Record email-like content during local capture verification.',
        'medium',
        'shadow_log',
        true,
        10,
        '{}'::jsonb
    from detector
    returning id, bundle_id
),
rule_detector as (
    insert into policy_rule_detectors (rule_id, detector_id)
    select rule.id, detector.id
    from rule
    join detector on detector.bundle_id = rule.bundle_id
    returning rule_id
),
route_policy as (
    insert into route_policy_bindings (route_id, bundle_id, enabled, priority)
    select route.id, bundle.id, true, 10
    from route
    join bundle on bundle.revision_id = route.revision_id
    returning route_id
),
route_verdict as (
    insert into route_verdict_provider_bindings (
        route_id,
        verdict_provider_id,
        enabled,
        execution_mode,
        priority
    )
    select route.id, verdict_provider.id, true, 'inline', 10
    from route
    join verdict_provider on verdict_provider.revision_id = route.revision_id
    returning route_id
),
route_limit as (
    insert into route_limit_policies (
        revision_id,
        route_id,
        requests_per_window,
        window_seconds,
        max_concurrent_requests,
        max_body_bytes,
        enabled
    )
    select revision_id, id, 60, 60, 10, 1048576, true
    from route
    returning route_id
),
sink as (
    insert into sinks (revision_id, name, kind, enabled, config)
    select
        id,
        'clickhouse',
        'clickhouse',
        true,
        '{"database":"kiwiguard","table":"kiwiguard_traffic_events"}'::jsonb
    from revision
    returning id, revision_id
),
retention as (
    insert into retention_policies (
        revision_id,
        name,
        sink_id,
        event_type,
        retention_days
    )
    select revision_id, 'traffic-30d', id, '*', 30
    from sink
    returning id
),
raw_capture as (
    insert into raw_capture_policies (
        revision_id,
        name,
        route_id,
        direction,
        enabled,
        sample_rate,
        redaction_mode
    )
    select route.revision_id, 'dev-full-payload-capture', route.id, 'both', true, 1.0000, 'none'
    from route
    returning id
),
snapshot as (
    insert into compiled_snapshots (
        revision_id,
        snapshot_hash,
        storage_ref,
        status,
        error_details,
        compiled_at
    )
    select id, 'dev-seed', 'local', 'compiled', '[]'::jsonb, now()
    from revision
    returning id, revision_id
)
insert into policy_activation_records (
    revision_id,
    snapshot_id,
    actor,
    status,
    reason
)
select revision_id, id, 'dev-env', 'active', 'local development seed'
from snapshot;

commit;
