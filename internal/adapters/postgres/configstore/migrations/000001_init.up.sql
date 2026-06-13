create extension if not exists pgcrypto;

create table config_revisions (
    id uuid primary key default gen_random_uuid(),
    revision_number bigserial not null unique,
    source text not null,
    status text not null check (status in ('draft', 'active', 'rejected')),
    summary text not null default '',
    created_at timestamptz not null default now(),
    activated_at timestamptz
);

create table routes (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    path_prefix text not null,
    upstream_provider text not null,
    upstream_model text not null,
    execution_mode text not null check (execution_mode in ('inline', 'async_shadow')),
    fallback_action text not null check (fallback_action in ('allow', 'block', 'shadow_log')),
    created_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table providers (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    base_url text not null,
    credential_ref text not null,
    timeout_ms integer not null check (timeout_ms > 0),
    created_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table verdict_providers (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    name text not null,
    adapter text not null check (adapter in ('http', 'grpc', 'openai_compatible')),
    endpoint text not null,
    timeout_ms integer not null check (timeout_ms > 0),
    created_at timestamptz not null default now(),
    unique (revision_id, name)
);

create table audit_events (
    id uuid primary key default gen_random_uuid(),
    actor text not null,
    action text not null,
    target_type text not null,
    target_id text not null,
    details jsonb not null default '{}'::jsonb,
    created_at timestamptz not null default now()
);
