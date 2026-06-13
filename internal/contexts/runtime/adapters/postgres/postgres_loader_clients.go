package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
)

// loadRuntimeClientsAndLimits hydrates gateway clients and effective limit policies for one runtime revision.
func loadRuntimeClientsAndLimits(ctx context.Context, q revisionstore.Queryer, revisionID string, cfg *postgresRuntimeConfig) error {
	var err error
	if cfg.GatewayClients, err = clientstore.LoadGatewayClients(ctx, q); err != nil {
		return err
	}
	if cfg.RouteLimitPolicies, err = limitstore.LoadRoutePolicies(ctx, q, revisionID); err != nil {
		return err
	}
	cfg.ClientRouteLimitOverrides, err = limitstore.LoadClientRouteOverrides(ctx, q, revisionID, "")
	return err
}
