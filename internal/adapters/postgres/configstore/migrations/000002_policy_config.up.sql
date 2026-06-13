alter table config_revisions
    add column actor text not null default 'system',
    add column validation_status text not null default 'valid' check (validation_status in ('pending', 'valid', 'invalid')),
    add column validation_errors jsonb not null default '[]'::jsonb,
    add column compiled_snapshot_hash text not null default '',
    add column compiled_snapshot_ref text not null default '';

alter table routes
    add column enabled boolean not null default true,
    add column priority integer not null default 100 check (priority >= 0),
    add column method text not null default 'POST',
    add column path text not null default '/v1/chat/completions',
    add column model_mapping_id uuid;

alter table providers
    add column provider_type text not null default 'openai_compatible' check (provider_type in ('openai_compatible', 'anthropic_compatible', 'http')),
    add column retry_config jsonb not null default '{}'::jsonb,
    add column circuit_breaker_config jsonb not null default '{}'::jsonb,
    add column headers jsonb not null default '{}'::jsonb,
    add column capabilities jsonb not null default '{}'::jsonb;

alter table verdict_providers
    add column credential_ref text not null default '',
    add column adapter_config jsonb not null default '{}'::jsonb,
    add column model_name text not null default '',
    add column retry_config jsonb not null default '{}'::jsonb,
    add column circuit_breaker_config jsonb not null default '{}'::jsonb,
    add column max_concurrency integer not null default 16 check (max_concurrency > 0);

create table model_mappings (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    source_model text not null,
    target_provider_id uuid references providers(id) on delete restrict,
    target_model text not null,
    parameters jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, name),
    unique (revision_id, source_model)
);

alter table routes
    add constraint routes_model_mapping_id_fkey foreign key (model_mapping_id) references model_mappings(id) on delete set null;

create table policy_bundles (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    source text not null check (source in ('built_in', 'user', 'imported')),
    version text not null default '',
    description text not null default '',
    default_action text not null default 'allow' check (default_action in ('allow', 'block', 'redact', 'shadow_log')),
    enabled boolean not null default true,
    metadata jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table policy_detectors (
    id uuid primary key default gen_random_uuid(),
    bundle_id uuid not null references policy_bundles(id) on delete cascade,
    name text not null,
    detector_type text not null check (detector_type in ('regex', 'dictionary', 'builtin', 'model')),
    pattern text not null default '',
    config jsonb not null default '{}'::jsonb,
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (bundle_id, name)
);

create table policy_rules (
    id uuid primary key default gen_random_uuid(),
    bundle_id uuid not null references policy_bundles(id) on delete cascade,
    name text not null,
    description text not null default '',
    severity text not null check (severity in ('low', 'medium', 'high', 'critical')),
    action text not null check (action in ('allow', 'block', 'redact', 'shadow_log')),
    enabled boolean not null default true,
    priority integer not null default 100 check (priority >= 0),
    config jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (bundle_id, name)
);

create table policy_rule_detectors (
    id uuid primary key default gen_random_uuid(),
    rule_id uuid not null references policy_rules(id) on delete cascade,
    detector_id uuid not null references policy_detectors(id) on delete cascade,
    match_mode text not null default 'any' check (match_mode in ('any', 'all')),
    created_at timestamptz not null default now(),
    unique (rule_id, detector_id)
);

create table policy_rule_scopes (
    id uuid primary key default gen_random_uuid(),
    rule_id uuid not null references policy_rules(id) on delete cascade,
    route_id uuid references routes(id) on delete cascade,
    provider_id uuid references providers(id) on delete cascade,
    model text not null default '',
    direction text not null default 'both' check (direction in ('request', 'response', 'both')),
    created_at timestamptz not null default now(),
    unique (rule_id, route_id, provider_id, model, direction)
);

create table route_policy_bindings (
    id uuid primary key default gen_random_uuid(),
    route_id uuid not null references routes(id) on delete cascade,
    bundle_id uuid not null references policy_bundles(id) on delete cascade,
    enabled boolean not null default true,
    priority integer not null default 100 check (priority >= 0),
    created_at timestamptz not null default now(),
    unique (route_id, bundle_id)
);

create table route_verdict_provider_bindings (
    id uuid primary key default gen_random_uuid(),
    route_id uuid not null references routes(id) on delete cascade,
    verdict_provider_id uuid not null references verdict_providers(id) on delete cascade,
    enabled boolean not null default true,
    execution_mode text not null default 'inline' check (execution_mode in ('inline', 'async_shadow')),
    priority integer not null default 100 check (priority >= 0),
    created_at timestamptz not null default now(),
    unique (route_id, verdict_provider_id)
);

create table sinks (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    kind text not null check (kind in ('clickhouse', 'webhook', 'stdout')),
    enabled boolean not null default true,
    config jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table retention_policies (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    sink_id uuid references sinks(id) on delete cascade,
    event_type text not null default '*',
    retention_days integer not null check (retention_days > 0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table raw_capture_policies (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    route_id uuid references routes(id) on delete cascade,
    direction text not null default 'both' check (direction in ('request', 'response', 'both')),
    enabled boolean not null default false,
    sample_rate numeric(5, 4) not null default 0 check (sample_rate >= 0 and sample_rate <= 1),
    redaction_mode text not null default 'redacted' check (redaction_mode in ('none', 'redacted', 'metadata_only')),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table compiled_snapshots (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    snapshot_hash text not null,
    storage_ref text not null default '',
    status text not null check (status in ('pending', 'compiled', 'failed')),
    error_details jsonb not null default '[]'::jsonb,
    compiled_at timestamptz,
    created_at timestamptz not null default now(),
    unique (revision_id, snapshot_hash)
);

create table policy_activation_records (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    snapshot_id uuid references compiled_snapshots(id) on delete set null,
    actor text not null,
    status text not null check (status in ('active', 'rejected', 'failed')),
    reason text not null default '',
    activated_at timestamptz not null default now(),
    created_at timestamptz not null default now()
);
