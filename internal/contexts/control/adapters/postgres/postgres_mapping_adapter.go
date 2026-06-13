package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// modelMappingParams preserves the historical JSON fields persisted alongside model mapping rows.
type modelMappingParams struct {
	RouteKey string `json:"route_key,omitempty"`
	Provider string `json:"provider,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// ListModelMappings returns persisted model mappings as control-plane contracts.
func (s postgresPolicyStore) ListModelMappings(ctx context.Context) ([]appcontrol.ModelMapping, error) {
	mappings, err := s.repo.ListModelMappings(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		return []appcontrol.ModelMapping{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]appcontrol.ModelMapping, 0, len(mappings))
	for _, mapping := range mappings {
		params := modelMappingParams{Enabled: true}
		if len(mapping.Parameters) > 0 {
			if err := json.Unmarshal(mapping.Parameters, &params); err != nil {
				return nil, fmt.Errorf("decode model mapping parameters: %w", err)
			}
		}
		items = append(items, appcontrol.ModelMapping{
			ID:       mapping.Name,
			RouteKey: params.RouteKey,
			Provider: params.Provider,
			Model:    mapping.TargetModel,
			Enabled:  params.Enabled,
		})
	}
	return items, nil
}

// PutModelMapping upserts one model mapping into PostgreSQL-backed control storage.
func (s postgresPolicyStore) PutModelMapping(ctx context.Context, mapping appcontrol.ModelMapping) error {
	params, err := json.Marshal(modelMappingParams{
		RouteKey: mapping.RouteKey,
		Provider: mapping.Provider,
		Enabled:  mapping.Enabled,
	})
	if err != nil {
		return fmt.Errorf("encode model mapping parameters: %w", err)
	}
	return s.repo.UpsertModelMapping(ctx, routingstore.ModelMapping{
		Name:        mapping.ID,
		SourceModel: mapping.Model,
		TargetModel: mapping.Model,
		Parameters:  params,
	})
}
