package resources

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/howmuchsec/kiwiguard/internal/config"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
	controlpostgres "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/postgres"
	kgruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime"
	postgresruntime "github.com/howmuchsec/kiwiguard/internal/contexts/runtime/adapters/postgres"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/clickhouse"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Production contains infrastructure resources opened for production assembly.
type Production struct {
	Pool           *pgxpool.Pool
	ClickHouseConn ch.Conn
	ControlStore   controlhttp.PolicyStore
	RuntimeRepo    kgruntime.RuntimeConfigRepository
}

// OpenProduction opens PostgreSQL and ClickHouse-backed production resources.
func OpenProduction(ctx context.Context, cfg config.Config) (Production, error) {
	if err := configstore.RunMigrations(cfg.PostgresDSN); err != nil {
		return Production{}, err
	}

	pool, err := configstore.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		return Production{}, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return Production{}, fmt.Errorf("ping postgres: %w", err)
	}

	conn, err := clickhouse.Open(clickhouse.Options{
		Addr:     cfg.ClickHouseAddr,
		Database: cfg.ClickHouseDatabase,
		Username: cfg.ClickHouseUsername,
		Password: cfg.ClickHousePassword,
	})
	if err != nil {
		pool.Close()
		return Production{}, err
	}
	if err := clickhouse.ProbeHealth(ctx, conn); err != nil {
		_ = conn.Close()
		pool.Close()
		return Production{}, err
	}

	return Production{
		Pool:           pool,
		ClickHouseConn: conn,
		ControlStore:   controlpostgres.NewStore(pool),
		RuntimeRepo:    postgresruntime.NewRepositoryFromPool(pool),
	}, nil
}

// Close releases opened production resources.
func (p Production) Close() error {
	if p.ClickHouseConn != nil {
		if err := p.ClickHouseConn.Close(); err != nil {
			return fmt.Errorf("close clickhouse: %w", err)
		}
	}
	if p.Pool != nil {
		p.Pool.Close()
	}
	return nil
}
