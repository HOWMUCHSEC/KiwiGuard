package postgres

import (
	"context"

	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

// ListRouteLimits returns default route limit policies from PostgreSQL-backed control storage.
func (s postgresPolicyStore) ListRouteLimits(ctx context.Context) ([]appcontrol.RouteLimit, error) {
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	routeNames := routeNamesByID(routes)
	policies, err := s.repo.ListRouteLimitPolicies(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appcontrol.RouteLimit, 0, len(policies))
	for _, policy := range policies {
		items = append(items, routeLimitFromPostgres(policy, routeNames))
	}
	return items, nil
}

// PutRouteLimit upserts one default route limit policy into PostgreSQL-backed control storage.
func (s postgresPolicyStore) PutRouteLimit(ctx context.Context, limit appcontrol.RouteLimit) (appcontrol.RouteLimit, error) {
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return appcontrol.RouteLimit{}, err
	}
	routeID, err := requiredIDByName(limit.RouteKey, routeIDsByName(routes), "route")
	if err != nil {
		return appcontrol.RouteLimit{}, err
	}
	policy := routeLimitToPostgres(limit, routeID)
	if err := s.repo.UpsertRouteLimitPolicy(ctx, policy); err != nil {
		return appcontrol.RouteLimit{}, err
	}
	return routeLimitFromPostgres(policy, routeNamesByID(routes)), nil
}

func (s postgresPolicyStore) ListClientRouteLimits(ctx context.Context, clientID string) ([]appcontrol.ClientRouteLimit, error) {
	clients, err := s.repo.ListGatewayClients(ctx)
	if err != nil {
		return nil, err
	}
	postgresClientID, err := requiredClientIDByExternalID(clientID, clients)
	if err != nil {
		return nil, err
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	overrides, err := s.repo.ListClientRouteLimitOverrides(ctx, postgresClientID)
	if err != nil {
		return nil, err
	}
	routeNames := routeNamesByID(routes)
	clientExternalIDs := clientExternalIDsByID(clients)
	items := make([]appcontrol.ClientRouteLimit, 0, len(overrides))
	for _, override := range overrides {
		items = append(items, clientRouteLimitFromPostgres(override, routeNames, clientExternalIDs))
	}
	return items, nil
}

func (s postgresPolicyStore) PutClientRouteLimit(ctx context.Context, limit appcontrol.ClientRouteLimit) (appcontrol.ClientRouteLimit, error) {
	clients, err := s.repo.ListGatewayClients(ctx)
	if err != nil {
		return appcontrol.ClientRouteLimit{}, err
	}
	postgresClientID, err := requiredClientIDByExternalID(limit.ClientID, clients)
	if err != nil {
		return appcontrol.ClientRouteLimit{}, err
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return appcontrol.ClientRouteLimit{}, err
	}
	routeID, err := requiredIDByName(limit.RouteKey, routeIDsByName(routes), "route")
	if err != nil {
		return appcontrol.ClientRouteLimit{}, err
	}
	override := clientRouteLimitToPostgres(limit, postgresClientID, routeID)
	if err := s.repo.UpsertClientRouteLimitOverride(ctx, override); err != nil {
		return appcontrol.ClientRouteLimit{}, err
	}
	return clientRouteLimitFromPostgres(override, routeNamesByID(routes), clientExternalIDsByID(clients)), nil
}

func (s postgresPolicyStore) DeleteClientRouteLimit(ctx context.Context, clientID, routeKey string) error {
	clients, err := s.repo.ListGatewayClients(ctx)
	if err != nil {
		return err
	}
	postgresClientID, err := requiredClientIDByExternalID(clientID, clients)
	if err != nil {
		return err
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return err
	}
	routeID, err := requiredIDByName(routeKey, routeIDsByName(routes), "route")
	if err != nil {
		return err
	}
	return s.repo.DeleteClientRouteLimitOverride(ctx, postgresClientID, routeID)
}

func routeLimitFromPostgres(policy limitstore.RoutePolicy, routeNames map[string]string) appcontrol.RouteLimit {
	return appcontrol.RouteLimit{
		RouteKey:              routeNames[policy.RouteID],
		RequestsPerWindow:     policy.RequestsPerWindow,
		WindowSeconds:         policy.WindowSeconds,
		MaxConcurrentRequests: policy.MaxConcurrentRequests,
		MaxBodyBytes:          policy.MaxBodyBytes,
		Enabled:               policy.Enabled,
	}
}

func routeLimitToPostgres(limit appcontrol.RouteLimit, routeID string) limitstore.RoutePolicy {
	return limitstore.RoutePolicy{
		RouteID:               routeID,
		RequestsPerWindow:     limit.RequestsPerWindow,
		WindowSeconds:         limit.WindowSeconds,
		MaxConcurrentRequests: limit.MaxConcurrentRequests,
		MaxBodyBytes:          limit.MaxBodyBytes,
		Enabled:               limit.Enabled,
	}
}

func clientRouteLimitFromPostgres(override limitstore.ClientRouteOverride, routeNames map[string]string, clientExternalIDs map[string]string) appcontrol.ClientRouteLimit {
	return appcontrol.ClientRouteLimit{
		ClientID:              clientExternalIDs[override.ClientID],
		RouteKey:              routeNames[override.RouteID],
		RequestsPerWindow:     override.RequestsPerWindow,
		WindowSeconds:         override.WindowSeconds,
		MaxConcurrentRequests: override.MaxConcurrentRequests,
		MaxBodyBytes:          override.MaxBodyBytes,
		Enabled:               override.Enabled,
	}
}

func clientRouteLimitToPostgres(limit appcontrol.ClientRouteLimit, clientID, routeID string) limitstore.ClientRouteOverride {
	return limitstore.ClientRouteOverride{
		ClientID:              clientID,
		RouteID:               routeID,
		RequestsPerWindow:     limit.RequestsPerWindow,
		WindowSeconds:         limit.WindowSeconds,
		MaxConcurrentRequests: limit.MaxConcurrentRequests,
		MaxBodyBytes:          limit.MaxBodyBytes,
		Enabled:               limit.Enabled,
	}
}
