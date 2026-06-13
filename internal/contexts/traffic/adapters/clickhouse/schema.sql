create database if not exists kiwiguard;

use kiwiguard;

create table if not exists kiwiguard_traffic_events
(
    event_time DateTime64(3, 'UTC'),
    event_id String,
    event_schema_version LowCardinality(String),
    config_revision_number Int64,
    snapshot_hash String,
    request_id String,
    correlation_id String,
    route_id String,
    provider_id String,
    verdict_provider_id String,
    policy_bundle_ids Array(String),
    http_method LowCardinality(String),
    api_path String,
    endpoint_kind LowCardinality(String),
    requested_model String,
    upstream_status UInt16,
    gateway_status UInt16,
    error_type LowCardinality(String),
    block_reason String,
    fallback_triggered Bool,
    risk_level LowCardinality(String),
    verdict_categories Array(String),
    confidence Float64,
    detector_categories Array(String),
    matched_span_count UInt32,
    streaming_chunk_count UInt32,
    redaction_count UInt32,
    termination_reason LowCardinality(String),
    gateway_latency_ms UInt32,
    detector_latency_ms UInt32,
    verdict_latency_ms UInt32,
    upstream_latency_ms UInt32,
    queue_delay_ms UInt32,
    total_latency_ms UInt32,
    sink_status LowCardinality(String),
    retry_count UInt16,
    spool_status LowCardinality(String),
    dropped Bool,
    drop_reason LowCardinality(String),
    raw_capture_policy_id String,
    retention_policy_id String,
    capture_reference String,
    route String,
    provider String,
    upstream_model String,
    mapped_model String,
    direction LowCardinality(String),
    verdict LowCardinality(String),
    action LowCardinality(String),
    detector_hits Array(String),
    policy_rule_ids Array(String),
    latency_ms UInt32,
    streaming_terminated Bool,
    raw_capture_enabled Bool,
    request_hash String,
    response_hash String,
    request_payload String,
    response_payload String,
    metadata_json String
)
engine = MergeTree
partition by toDate(event_time)
order by (route_id, event_time, request_id)
ttl toDateTime(event_time) + interval 30 day;

alter table kiwiguard_traffic_events
    add column if not exists request_payload String after response_hash;

alter table kiwiguard_traffic_events
    add column if not exists response_payload String after request_payload;
