alter table policy_bundles
    add column if not exists default_action text not null default 'allow' check (default_action in ('allow', 'block', 'redact', 'shadow_log'));
