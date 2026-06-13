package postgres

import (
	"context"
	"errors"
	"fmt"

	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	clients "github.com/howmuchsec/kiwiguard/internal/contexts/clients/domain"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

// ListGatewayClients returns all persisted gateway clients as control-plane contracts.
func (s postgresPolicyStore) ListGatewayClients(ctx context.Context) ([]appcontrol.GatewayClient, error) {
	clients, err := s.repo.ListGatewayClients(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appcontrol.GatewayClient, 0, len(clients))
	for _, client := range clients {
		items = append(items, gatewayClientFromPostgres(client))
	}
	return items, nil
}

// CreateGatewayClient generates and persists a new gateway client key.
func (s postgresPolicyStore) CreateGatewayClient(ctx context.Context, request appcontrol.CreateGatewayClientRequest) (appcontrol.CreateGatewayClientResponse, error) {
	if request.ID != "" {
		if _, err := s.findGatewayClient(ctx, request.ID); err == nil {
			return appcontrol.CreateGatewayClientResponse{}, errGatewayClientAlreadyExists
		} else if !errors.Is(err, errGatewayClientNotFound) {
			return appcontrol.CreateGatewayClientResponse{}, err
		}
	}
	for request.ID == "" {
		id, err := randomID("client")
		if err != nil {
			return appcontrol.CreateGatewayClientResponse{}, fmt.Errorf("generate gateway client id: %w", err)
		}
		if _, err := s.findGatewayClient(ctx, id); err == nil {
			continue
		} else if !errors.Is(err, errGatewayClientNotFound) {
			return appcontrol.CreateGatewayClientResponse{}, err
		}
		request.ID = id
	}
	key, material, err := clients.GenerateKey(request.ID)
	if err != nil {
		return appcontrol.CreateGatewayClientResponse{}, err
	}
	client := clientstore.GatewayClient{
		ExternalID: request.ID,
		Name:       request.Name,
		Status:     request.Status,
		KeyPrefix:  material.Prefix,
		KeyHash:    material.Hash,
		Notes:      request.Notes,
	}
	if err := s.repo.CreateGatewayClient(ctx, client); errors.Is(err, clientstore.ErrAlreadyExists) {
		return appcontrol.CreateGatewayClientResponse{}, errGatewayClientAlreadyExists
	} else if err != nil {
		return appcontrol.CreateGatewayClientResponse{}, err
	}
	persisted, err := s.findGatewayClient(ctx, request.ID)
	if err != nil {
		return appcontrol.CreateGatewayClientResponse{}, err
	}
	if persisted.Status != request.Status {
		return appcontrol.CreateGatewayClientResponse{}, errGatewayClientAlreadyExists
	}
	return appcontrol.CreateGatewayClientResponse{
		Client: gatewayClientFromPostgres(persisted),
		Key:    key,
	}, nil
}

func (s postgresPolicyStore) PatchGatewayClient(ctx context.Context, client appcontrol.GatewayClient) (appcontrol.GatewayClient, error) {
	existing, err := s.findGatewayClient(ctx, client.ID)
	if err != nil {
		return appcontrol.GatewayClient{}, err
	}
	existing.Name = client.Name
	existing.Status = client.Status
	existing.Notes = client.Notes
	if err := s.repo.UpsertGatewayClient(ctx, existing); err != nil {
		return appcontrol.GatewayClient{}, err
	}
	persisted, err := s.findGatewayClient(ctx, client.ID)
	if err != nil {
		return appcontrol.GatewayClient{}, err
	}
	return gatewayClientFromPostgres(persisted), nil
}

func (s postgresPolicyStore) RevokeGatewayClient(ctx context.Context, clientID string) (appcontrol.GatewayClient, error) {
	if _, err := s.findGatewayClient(ctx, clientID); err != nil {
		return appcontrol.GatewayClient{}, err
	}
	if err := s.repo.RevokeGatewayClient(ctx, clientID); err != nil {
		return appcontrol.GatewayClient{}, err
	}
	client, err := s.findGatewayClient(ctx, clientID)
	if err != nil {
		return appcontrol.GatewayClient{}, err
	}
	client.Status = string(clients.StatusRevoked)
	return gatewayClientFromPostgres(client), nil
}

func (s postgresPolicyStore) findGatewayClient(ctx context.Context, clientID string) (clientstore.GatewayClient, error) {
	clients, err := s.repo.ListGatewayClients(ctx)
	if err != nil {
		return clientstore.GatewayClient{}, err
	}
	for _, client := range clients {
		if client.ExternalID == clientID || client.ID == clientID {
			return client, nil
		}
	}
	return clientstore.GatewayClient{}, errGatewayClientNotFound
}

func gatewayClientFromPostgres(client clientstore.GatewayClient) appcontrol.GatewayClient {
	return appcontrol.GatewayClient{
		ID:        client.ExternalID,
		Name:      client.Name,
		Status:    client.Status,
		KeyPrefix: client.KeyPrefix,
		Notes:     client.Notes,
	}
}
