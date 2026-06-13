package configstore

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func newMigratedPostgresPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	skipIfTestcontainersUnavailable(t)

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("kiwiguard"),
		tcpostgres.WithUsername("kiwiguard"),
		tcpostgres.WithPassword("kiwiguard"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("postgres.Run() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := container.Terminate(cleanupCtx); err != nil {
			t.Errorf("container.Terminate() error = %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("container.ConnectionString() error = %v", err)
	}
	return newMigratedPostgresPoolFromDSN(t, ctx, dsn)
}

func newMigratedPostgresPoolFromDSN(t *testing.T, ctx context.Context, dsn string) *pgxpool.Pool {
	t.Helper()

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func routeIDByName(t *testing.T, ctx context.Context, repo *ConfigRepository, name string) string {
	t.Helper()

	routes, err := testListRoutes(ctx, repo)
	if err != nil {
		t.Fatalf("ListRoutes() error = %v", err)
	}
	for _, route := range routes {
		if route.Name == name {
			return route.ID
		}
	}
	t.Fatalf("route %q not found in %+v", name, routes)
	return ""
}

func seedRuntimeConfig(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var revisionID, providerID, mappingID, routeID, verdictID, bundleID, detectorID, ruleID, sinkID, snapshotID string
	err := pool.QueryRow(ctx, `
		insert into config_revisions (source, status, summary, actor, compiled_snapshot_hash, compiled_snapshot_ref, activated_at)
		values ('test', 'active', 'seed', 'seed', 'seed-hash', 'seed-ref', now())
		returning id
	`).Scan(&revisionID)
	if err != nil {
		t.Fatalf("insert config_revisions: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into providers (revision_id, name, base_url, credential_ref, timeout_ms, provider_type, headers)
		values ($1, 'openai', 'http://upstream.test', 'secret/openai', 30000, 'openai_compatible', '{"x-test":"true"}')
		returning id
	`, revisionID).Scan(&providerID)
	if err != nil {
		t.Fatalf("insert providers: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into model_mappings (revision_id, name, source_model, target_provider_id, target_model, parameters)
		values ($1, 'default', 'gpt-test', $2, 'gpt-4o-mini', '{"temperature":0}')
		returning id
	`, revisionID, providerID).Scan(&mappingID)
	if err != nil {
		t.Fatalf("insert model_mappings: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into routes (revision_id, name, path_prefix, upstream_provider, upstream_model, execution_mode, fallback_action, method, path, model_mapping_id)
		values ($1, 'chat', '/v1/chat/completions', 'openai', 'gpt-4o-mini', 'inline', 'allow', 'POST', '/v1/chat/completions', $2)
		returning id
	`, revisionID, mappingID).Scan(&routeID)
	if err != nil {
		t.Fatalf("insert routes: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into verdict_providers (revision_id, name, adapter, endpoint, timeout_ms, credential_ref, model_name, max_concurrency)
		values ($1, 'sec-model', 'http', 'http://verdict.test/evaluate', 5000, 'secret/verdict', 'kg-sec', 16)
		returning id
	`, revisionID).Scan(&verdictID)
	if err != nil {
		t.Fatalf("insert verdict_providers: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into policy_bundles (revision_id, name, source, version, description, enabled)
		values ($1, 'pii', 'user', '2026.05', 'PII rules', true)
		returning id
	`, revisionID).Scan(&bundleID)
	if err != nil {
		t.Fatalf("insert policy_bundles: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into policy_detectors (bundle_id, name, detector_type, pattern, config, enabled)
		values ($1, 'email', 'regex', '[a-z]+@[a-z]+\\.com', '{"categories":["pii.email"]}', true)
		returning id
	`, bundleID).Scan(&detectorID)
	if err != nil {
		t.Fatalf("insert policy_detectors: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into policy_rules (bundle_id, name, description, severity, action, enabled, priority)
		values ($1, 'block-email', 'block email', 'high', 'block', true, 10)
		returning id
	`, bundleID).Scan(&ruleID)
	if err != nil {
		t.Fatalf("insert policy_rules: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into policy_rule_detectors (rule_id, detector_id) values ($1, $2)`, ruleID, detectorID); err != nil {
		t.Fatalf("insert policy_rule_detectors: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into policy_rule_scopes (rule_id, route_id, provider_id, model, direction) values ($1, $2, $3, 'gpt-test', 'request')`, ruleID, routeID, providerID); err != nil {
		t.Fatalf("insert policy_rule_scopes: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into route_policy_bindings (route_id, bundle_id, priority) values ($1, $2, 10)`, routeID, bundleID); err != nil {
		t.Fatalf("insert route_policy_bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into route_verdict_provider_bindings (route_id, verdict_provider_id, execution_mode, priority) values ($1, $2, 'inline', 10)`, routeID, verdictID); err != nil {
		t.Fatalf("insert route_verdict_provider_bindings: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into sinks (revision_id, name, kind, config)
		values ($1, 'events', 'clickhouse', '{"database":"kiwiguard"}')
		returning id
	`, revisionID).Scan(&sinkID)
	if err != nil {
		t.Fatalf("insert sinks: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into retention_policies (revision_id, name, sink_id, event_type, retention_days) values ($1, 'events-30d', $2, '*', 30)`, revisionID, sinkID); err != nil {
		t.Fatalf("insert retention_policies: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into raw_capture_policies (revision_id, name, route_id, direction, enabled, sample_rate, redaction_mode) values ($1, 'redacted-sample', $2, 'both', true, 0.2500, 'redacted')`, revisionID, routeID); err != nil {
		t.Fatalf("insert raw_capture_policies: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into compiled_snapshots (revision_id, snapshot_hash, storage_ref, status, compiled_at)
		values ($1, 'seed-hash', 'seed-ref', 'compiled', now())
		returning id
	`, revisionID).Scan(&snapshotID)
	if err != nil {
		t.Fatalf("insert compiled_snapshots: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into policy_activation_records (revision_id, snapshot_id, actor, status, reason) values ($1, $2, 'seed', 'active', 'seed')`, revisionID, snapshotID); err != nil {
		t.Fatalf("insert policy_activation_records: %v", err)
	}
}

func seedOpenAIRoute(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var revisionID, mappingID string
	err := pool.QueryRow(ctx, `
		select r.id::text, m.id::text
		from config_revisions r
		join model_mappings m on m.revision_id = r.id
		where r.status = 'active'
		order by r.revision_number desc
		limit 1
	`).Scan(&revisionID, &mappingID)
	if err != nil {
		t.Fatalf("load active revision for openai route: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into routes (revision_id, name, path_prefix, upstream_provider, upstream_model, execution_mode, fallback_action, method, path, model_mapping_id, priority)
		values ($1, 'openai', '/v1/openai', 'openai', 'gpt-4o-mini', 'inline', 'allow', 'POST', '/v1/openai', $2, 20)
	`, revisionID, mappingID); err != nil {
		t.Fatalf("insert openai route: %v", err)
	}
}

func seedGatewayClientAndLimits(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	var revisionID, routeID, clientID string
	err := pool.QueryRow(ctx, `
		select r.revision_id::text, r.id::text
		from routes r
		join config_revisions cr on cr.id = r.revision_id
		where cr.status = 'active' and r.name = 'openai'
		order by cr.revision_number desc
		limit 1
	`).Scan(&revisionID, &routeID)
	if err != nil {
		t.Fatalf("load active openai route: %v", err)
	}
	err = pool.QueryRow(ctx, `
		insert into gateway_clients (external_id, name, status, key_prefix, key_hash, notes)
		values ('client-acme', 'Acme Console', 'enabled', 'kg_acme', 'sha256:acme', 'primary test client')
		returning id::text
	`).Scan(&clientID)
	if err != nil {
		t.Fatalf("insert gateway client: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into route_limit_policies (
			revision_id, route_id, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		)
		values ($1, $2, 120, 60, 8, 1048576, true)
	`, revisionID, routeID); err != nil {
		t.Fatalf("insert route limit policy: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		insert into client_route_limit_overrides (
			revision_id, client_id, route_id, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		)
		values ($1, $2, $3, 40, 60, 3, 262144, true)
	`, revisionID, clientID, routeID); err != nil {
		t.Fatalf("insert client route limit override: %v", err)
	}
}
