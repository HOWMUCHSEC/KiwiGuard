package limit

import (
	"context"
	"fmt"
)

// LoadRoutePolicies hydrates default route-limit policies attached to one config revision.
func LoadRoutePolicies(ctx context.Context, q Queryer, revisionID string) ([]RoutePolicy, error) {
	rows, err := q.Query(ctx, `
		select id::text, route_id::text, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		from route_limit_policies
		where revision_id = $1
		order by route_id
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load route limit policies: %w", err)
	}
	defer rows.Close()

	var policies []RoutePolicy
	for rows.Next() {
		var policy RoutePolicy
		if err := rows.Scan(&policy.ID, &policy.RouteID, &policy.RequestsPerWindow, &policy.WindowSeconds, &policy.MaxConcurrentRequests, &policy.MaxBodyBytes, &policy.Enabled); err != nil {
			return nil, fmt.Errorf("scan route limit policy: %w", err)
		}
		policies = append(policies, policy)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate route limit policies: %w", err)
	}
	return policies, nil
}

// LoadClientRouteOverrides hydrates per-client route-limit overrides attached to one config revision.
func LoadClientRouteOverrides(ctx context.Context, q Queryer, revisionID, clientID string) ([]ClientRouteOverride, error) {
	rows, err := q.Query(ctx, `
		select id::text, client_id::text, route_id::text, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		from client_route_limit_overrides
		where revision_id = $1 and ($2 = '' or client_id::text = $2)
		order by client_id, route_id
	`, revisionID, clientID)
	if err != nil {
		return nil, fmt.Errorf("load client route limit overrides: %w", err)
	}
	defer rows.Close()

	var overrides []ClientRouteOverride
	for rows.Next() {
		var override ClientRouteOverride
		if err := rows.Scan(&override.ID, &override.ClientID, &override.RouteID, &override.RequestsPerWindow, &override.WindowSeconds, &override.MaxConcurrentRequests, &override.MaxBodyBytes, &override.Enabled); err != nil {
			return nil, fmt.Errorf("scan client route limit override: %w", err)
		}
		overrides = append(overrides, override)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate client route limit overrides: %w", err)
	}
	return overrides, nil
}
