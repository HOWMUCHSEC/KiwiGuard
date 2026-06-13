package postgres

import (
	"context"
	"fmt"
)

// LoadGatewayClients hydrates the gateway-client credential records shared across active revisions.
func LoadGatewayClients(ctx context.Context, q Queryer) ([]GatewayClient, error) {
	rows, err := q.Query(ctx, `
		select id::text, external_id, name, status, key_prefix, key_hash, notes,
			created_at, updated_at, revoked_at
		from gateway_clients
		order by name, external_id
	`)
	if err != nil {
		return nil, fmt.Errorf("load gateway clients: %w", err)
	}
	defer rows.Close()

	var clients []GatewayClient
	for rows.Next() {
		var client GatewayClient
		if err := rows.Scan(&client.ID, &client.ExternalID, &client.Name, &client.Status, &client.KeyPrefix, &client.KeyHash, &client.Notes, &client.CreatedAt, &client.UpdatedAt, &client.RevokedAt); err != nil {
			return nil, fmt.Errorf("scan gateway client: %w", err)
		}
		clients = append(clients, client)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gateway clients: %w", err)
	}
	return clients, nil
}
