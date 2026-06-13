package limit

import (
	"context"
	"fmt"
)

// CloneGatewayRecords clones route and client route limits into a draft revision.
func CloneGatewayRecords(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	if err := CloneRoutePolicies(ctx, q, sourceRevisionID, draftRevisionID, routeIDs); err != nil {
		return err
	}
	return CloneClientRouteOverrides(ctx, q, sourceRevisionID, draftRevisionID, routeIDs)
}

// CloneRoutePolicies clones route limit policies into a draft revision.
func CloneRoutePolicies(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	type policyClone struct {
		policy     RoutePolicy
		oldRouteID string
	}
	rows, err := q.Query(ctx, `
		select route_id::text, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		from route_limit_policies
		where revision_id = $1
		order by route_id
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load route limit policies for draft clone: %w", err)
	}
	defer rows.Close()

	var items []policyClone
	for rows.Next() {
		var item policyClone
		if err := rows.Scan(&item.oldRouteID, &item.policy.RequestsPerWindow, &item.policy.WindowSeconds, &item.policy.MaxConcurrentRequests, &item.policy.MaxBodyBytes, &item.policy.Enabled); err != nil {
			return fmt.Errorf("scan route limit policy for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate route limit policies for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		routeID, err := remapRequiredID(item.oldRouteID, routeIDs, "route limit policy route")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into route_limit_policies (
				revision_id, route_id, requests_per_window, window_seconds,
				max_concurrent_requests, max_body_bytes, enabled
			)
			values ($1, $2, $3, $4, $5, $6, $7)
		`, draftRevisionID, routeID, item.policy.RequestsPerWindow, item.policy.WindowSeconds, item.policy.MaxConcurrentRequests, item.policy.MaxBodyBytes, item.policy.Enabled); err != nil {
			return fmt.Errorf("clone route limit policy: %w", err)
		}
	}
	return nil
}

// CloneClientRouteOverrides clones client route limit overrides into a draft revision.
func CloneClientRouteOverrides(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	type overrideClone struct {
		override   ClientRouteOverride
		oldRouteID string
	}
	rows, err := q.Query(ctx, `
		select client_id::text, route_id::text, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		from client_route_limit_overrides
		where revision_id = $1
		order by client_id, route_id
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load client route limit overrides for draft clone: %w", err)
	}
	defer rows.Close()

	var items []overrideClone
	for rows.Next() {
		var item overrideClone
		if err := rows.Scan(&item.override.ClientID, &item.oldRouteID, &item.override.RequestsPerWindow, &item.override.WindowSeconds, &item.override.MaxConcurrentRequests, &item.override.MaxBodyBytes, &item.override.Enabled); err != nil {
			return fmt.Errorf("scan client route limit override for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate client route limit overrides for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		routeID, err := remapRequiredID(item.oldRouteID, routeIDs, "client route limit override route")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into client_route_limit_overrides (
				revision_id, client_id, route_id, requests_per_window, window_seconds,
				max_concurrent_requests, max_body_bytes, enabled
			)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
		`, draftRevisionID, item.override.ClientID, routeID, item.override.RequestsPerWindow, item.override.WindowSeconds, item.override.MaxConcurrentRequests, item.override.MaxBodyBytes, item.override.Enabled); err != nil {
			return fmt.Errorf("clone client route limit override: %w", err)
		}
	}
	return nil
}

func remapRequiredID(oldID string, ids map[string]string, label string) (string, error) {
	if newID, ok := ids[oldID]; ok {
		return newID, nil
	}
	return "", fmt.Errorf("missing cloned %s id for %s", label, oldID)
}
