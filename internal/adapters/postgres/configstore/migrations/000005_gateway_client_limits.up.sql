create table gateway_clients (
    id uuid primary key default gen_random_uuid(),
    external_id text not null unique,
    name text not null,
    status text not null check (status in ('enabled', 'disabled', 'revoked')),
    key_prefix text not null unique,
    key_hash text not null,
    notes text not null default '',
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    revoked_at timestamptz
);

create table gateway_client_config_versions (
    id boolean primary key default true check (id),
    generation bigint not null default 0 check (generation >= 0),
    updated_at timestamptz not null default now()
);

insert into gateway_client_config_versions (id, generation)
values (true, 0);

create table route_limit_policies (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    route_id uuid not null references routes(id) on delete cascade,
    requests_per_window integer not null check (requests_per_window > 0),
    window_seconds integer not null check (window_seconds > 0),
    max_concurrent_requests integer not null check (max_concurrent_requests > 0),
    max_body_bytes bigint not null check (max_body_bytes > 0),
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, route_id)
);

create table client_route_limit_overrides (
    id uuid primary key default gen_random_uuid(),
    revision_id uuid not null references config_revisions(id) on delete cascade,
    client_id uuid not null references gateway_clients(id) on delete cascade,
    route_id uuid not null references routes(id) on delete cascade,
    requests_per_window integer not null check (requests_per_window > 0),
    window_seconds integer not null check (window_seconds > 0),
    max_concurrent_requests integer not null check (max_concurrent_requests > 0),
    max_body_bytes bigint not null check (max_body_bytes > 0),
    enabled boolean not null default true,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (revision_id, client_id, route_id)
);
