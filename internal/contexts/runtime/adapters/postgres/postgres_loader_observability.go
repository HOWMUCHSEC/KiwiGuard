package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

// loadRuntimeObservability hydrates observability sinks, retention rules, and raw-capture policy for one revision.
func loadRuntimeObservability(ctx context.Context, q revisionstore.Queryer, revisionID string, cfg *postgresRuntimeConfig) error {
	var err error
	if cfg.Sinks, err = observabilitystore.LoadSinks(ctx, q, revisionID); err != nil {
		return err
	}
	if cfg.Retention, err = observabilitystore.LoadRetentionPolicies(ctx, q, revisionID); err != nil {
		return err
	}
	cfg.RawCapture, err = observabilitystore.LoadRawCapturePolicies(ctx, q, revisionID)
	return err
}
