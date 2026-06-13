package configstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestRunMigrationsCreatesConfigRevisions(t *testing.T) {
	skipIfTestcontainersUnavailable(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations() second call error = %v", err)
	}

	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	for _, table := range []string{
		"config_revisions",
		"routes",
		"providers",
		"verdict_providers",
		"audit_events",
	} {
		var exists bool
		err = pool.QueryRow(ctx, `
		select exists (
			select 1
			from information_schema.tables
			where table_schema = 'public'
				and table_name = $1
		)
	`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("query %s existence: %v", table, err)
		}
		if !exists {
			t.Fatalf("%s table was not created", table)
		}
	}
}

func TestRunMigrationsCreatesPolicyConfigSchema(t *testing.T) {
	skipIfTestcontainersUnavailable(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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

	if err := RunMigrations(dsn); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	for _, table := range []string{
		"policy_bundles",
		"policy_detectors",
		"policy_rules",
		"policy_rule_detectors",
		"policy_rule_scopes",
		"route_policy_bindings",
		"route_verdict_provider_bindings",
		"model_mappings",
		"sinks",
		"retention_policies",
		"raw_capture_policies",
		"compiled_snapshots",
		"policy_activation_records",
	} {
		assertTableExists(t, ctx, pool, table)
	}

	assertColumnExists(t, ctx, pool, "config_revisions", "actor")
	assertColumnExists(t, ctx, pool, "config_revisions", "validation_status")
	assertColumnExists(t, ctx, pool, "config_revisions", "compiled_snapshot_hash")
	assertColumnExists(t, ctx, pool, "routes", "model_mapping_id")
	assertColumnExists(t, ctx, pool, "providers", "provider_type")
	assertColumnExists(t, ctx, pool, "verdict_providers", "max_concurrency")
	assertColumnExists(t, ctx, pool, "verdict_providers", "enabled")

	assertCheckConstraint(t, ctx, pool, "policy_bundles", "policy_bundles_source_check")
	assertCheckConstraint(t, ctx, pool, "policy_rules", "policy_rules_action_check")
	assertCheckConstraint(t, ctx, pool, "sinks", "sinks_kind_check")
	assertForeignKey(t, ctx, pool, "policy_rules", "policy_rules_bundle_id_fkey")
	assertForeignKey(t, ctx, pool, "route_policy_bindings", "route_policy_bindings_route_id_fkey")
}

func TestNotifierSubscriberReceivesActivatedRevision(t *testing.T) {
	skipIfTestcontainersUnavailable(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

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
	pool, err := NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	subscribeCtx, subscribeCancel := context.WithCancel(ctx)
	defer subscribeCancel()
	notifications, err := NewSubscriber(pool).Subscribe(subscribeCtx)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	const wantRevision int64 = 42
	if err := NewNotifier(pool).NotifyConfigActivated(ctx, wantRevision); err != nil {
		t.Fatalf("NotifyConfigActivated() error = %v", err)
	}

	select {
	case gotRevision := <-notifications:
		if gotRevision != wantRevision {
			t.Fatalf("revision = %d, want %d", gotRevision, wantRevision)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for config activation notification")
	}
}

func TestNewPoolReportsInvalidDSN(t *testing.T) {
	_, err := NewPool(context.Background(), "postgres://%")
	if err == nil || !strings.Contains(err.Error(), "create postgres pool") {
		t.Fatalf("NewPool() error = %v, want create postgres pool context", err)
	}
}

func TestRunMigrationsReportsInvalidDSN(t *testing.T) {
	err := RunMigrations("postgres://%")
	if err == nil || !strings.Contains(err.Error(), "migration") {
		t.Fatalf("RunMigrations() error = %v, want migration context", err)
	}
}

func assertTableExists(t *testing.T, ctx context.Context, pool queryRower, table string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		select exists (
			select 1
			from information_schema.tables
			where table_schema = 'public'
				and table_name = $1
		)
	`, table).Scan(&exists)
	if err != nil {
		t.Fatalf("query %s existence: %v", table, err)
	}
	if !exists {
		t.Fatalf("%s table was not created", table)
	}
}

func assertColumnExists(t *testing.T, ctx context.Context, pool queryRower, table, column string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		select exists (
			select 1
			from information_schema.columns
			where table_schema = 'public'
				and table_name = $1
				and column_name = $2
		)
	`, table, column).Scan(&exists)
	if err != nil {
		t.Fatalf("query %s.%s column existence: %v", table, column, err)
	}
	if !exists {
		t.Fatalf("%s.%s column was not created", table, column)
	}
}

func assertCheckConstraint(t *testing.T, ctx context.Context, pool queryRower, table, constraint string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		select exists (
			select 1
			from information_schema.table_constraints
			where table_schema = 'public'
				and table_name = $1
				and constraint_name = $2
				and constraint_type = 'CHECK'
		)
	`, table, constraint).Scan(&exists)
	if err != nil {
		t.Fatalf("query %s check constraint %s: %v", table, constraint, err)
	}
	if !exists {
		t.Fatalf("%s check constraint %s was not created", table, constraint)
	}
}

func assertForeignKey(t *testing.T, ctx context.Context, pool queryRower, table, constraint string) {
	t.Helper()

	var exists bool
	err := pool.QueryRow(ctx, `
		select exists (
			select 1
			from information_schema.table_constraints
			where table_schema = 'public'
				and table_name = $1
				and constraint_name = $2
				and constraint_type = 'FOREIGN KEY'
		)
	`, table, constraint).Scan(&exists)
	if err != nil {
		t.Fatalf("query %s foreign key %s: %v", table, constraint, err)
	}
	if !exists {
		t.Fatalf("%s foreign key %s was not created", table, constraint)
	}
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
