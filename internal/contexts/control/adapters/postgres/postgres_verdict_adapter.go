package postgres

import (
	"context"
	"errors"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// ListVerdictProviders returns persisted verdict providers as control-plane contracts.
func (s postgresPolicyStore) ListVerdictProviders(ctx context.Context) ([]appcontrol.VerdictProvider, error) {
	providers, err := s.repo.ListVerdictProviders(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		return []appcontrol.VerdictProvider{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]appcontrol.VerdictProvider, 0, len(providers))
	for _, provider := range providers {
		items = append(items, appcontrol.VerdictProvider{
			ID:            provider.Name,
			Name:          provider.Name,
			Endpoint:      provider.Endpoint,
			CredentialRef: provider.CredentialRef,
			Mode:          "inline",
			Enabled:       provider.Enabled,
		})
	}
	return items, nil
}

// PutVerdictProvider upserts one verdict provider into PostgreSQL-backed control storage.
func (s postgresPolicyStore) PutVerdictProvider(ctx context.Context, provider appcontrol.VerdictProvider) error {
	return s.repo.UpsertVerdictProvider(ctx, routingstore.VerdictProvider{
		Name:          firstNonEmpty(provider.Name, provider.ID),
		Adapter:       "http",
		Endpoint:      provider.Endpoint,
		CredentialRef: provider.CredentialRef,
		Enabled:       provider.Enabled,
	})
}
