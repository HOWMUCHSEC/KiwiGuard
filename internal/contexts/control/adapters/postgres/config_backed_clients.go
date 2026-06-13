package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
)

// ListGatewayClients reads the gateway-client set from shared client storage outside revision snapshots.
func (r *configBackedRepository) ListGatewayClients(ctx context.Context) ([]clientstore.GatewayClient, error) {
	var clients []clientstore.GatewayClient
	err := r.core.WithTransaction(ctx, "list gateway clients", func(ctx context.Context, q revisionstore.Queryer) error {
		var err error
		clients, err = clientstore.LoadGatewayClients(ctx, q)
		return err
	})
	return clients, err
}

func (r *configBackedRepository) CreateGatewayClient(ctx context.Context, client clientstore.GatewayClient) error {
	return r.core.WithTransaction(ctx, "create gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		if err := clientstore.Create(ctx, q, client); err != nil {
			return err
		}
		return clientstore.BumpGeneration(ctx, q, configstore.ConfigActivatedChannel)
	})
}

func (r *configBackedRepository) UpsertGatewayClient(ctx context.Context, client clientstore.GatewayClient) error {
	return r.core.WithTransaction(ctx, "upsert gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		if err := clientstore.Upsert(ctx, q, client); err != nil {
			return err
		}
		return clientstore.BumpGeneration(ctx, q, configstore.ConfigActivatedChannel)
	})
}

func (r *configBackedRepository) RevokeGatewayClient(ctx context.Context, clientID string) error {
	return r.core.WithTransaction(ctx, "revoke gateway client", func(ctx context.Context, q revisionstore.Queryer) error {
		changed, err := clientstore.Revoke(ctx, q, clientID)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		return clientstore.BumpGeneration(ctx, q, configstore.ConfigActivatedChannel)
	})
}
