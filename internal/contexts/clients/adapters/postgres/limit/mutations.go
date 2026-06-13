package limit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// UpsertRoutePolicy writes a route limit policy into a draft revision.
func UpsertRoutePolicy(ctx context.Context, q Queryer, revisionID string, policy RoutePolicy) error {
	_, err := q.Exec(ctx, `
		insert into route_limit_policies (
			revision_id, route_id, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		)
		values ($1, $2, $3, $4, $5, $6, $7)
		on conflict (revision_id, route_id) do update
		set requests_per_window = excluded.requests_per_window,
			window_seconds = excluded.window_seconds,
			max_concurrent_requests = excluded.max_concurrent_requests,
			max_body_bytes = excluded.max_body_bytes,
			enabled = excluded.enabled,
			updated_at = now()
	`, revisionID, policy.RouteID, policy.RequestsPerWindow, policy.WindowSeconds, policy.MaxConcurrentRequests, policy.MaxBodyBytes, policy.Enabled)
	if err != nil {
		return fmt.Errorf("upsert route limit policy: %w", err)
	}
	return nil
}

// UpsertClientRouteOverride writes a client route limit override into a draft revision.
func UpsertClientRouteOverride(ctx context.Context, q Queryer, revisionID string, override ClientRouteOverride) error {
	_, err := q.Exec(ctx, `
		insert into client_route_limit_overrides (
			revision_id, client_id, route_id, requests_per_window, window_seconds,
			max_concurrent_requests, max_body_bytes, enabled
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (revision_id, client_id, route_id) do update
		set requests_per_window = excluded.requests_per_window,
			window_seconds = excluded.window_seconds,
			max_concurrent_requests = excluded.max_concurrent_requests,
			max_body_bytes = excluded.max_body_bytes,
			enabled = excluded.enabled,
			updated_at = now()
	`, revisionID, override.ClientID, override.RouteID, override.RequestsPerWindow, override.WindowSeconds, override.MaxConcurrentRequests, override.MaxBodyBytes, override.Enabled)
	if err != nil {
		return fmt.Errorf("upsert client route limit override: %w", err)
	}
	return nil
}

// DeleteClientRouteOverride deletes a client route limit override from a draft revision.
func DeleteClientRouteOverride(ctx context.Context, q Queryer, revisionID, clientID, routeID string) error {
	if _, err := q.Exec(ctx, `
		delete from client_route_limit_overrides
		where revision_id = $1 and client_id::text = $2 and route_id::text = $3
	`, revisionID, clientID, routeID); err != nil {
		return fmt.Errorf("delete client route limit override: %w", err)
	}
	return nil
}

// RouteIDForRevision resolves a route ID to its cloned counterpart in the target revision.
func RouteIDForRevision(ctx context.Context, q Queryer, revisionID, routeID string) (string, error) {
	var currentID string
	err := q.QueryRow(ctx, `
		select id::text
		from routes
		where revision_id = $1 and id::text = $2
	`, revisionID, routeID).Scan(&currentID)
	if err == nil {
		return currentID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("resolve route for revision: %w", err)
	}

	var routeName string
	err = q.QueryRow(ctx, `select name from routes where id::text = $1`, routeID).Scan(&routeName)
	if errors.Is(err, pgx.ErrNoRows) {
		return routeID, nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve route name: %w", err)
	}

	err = q.QueryRow(ctx, `
		select id::text
		from routes
		where revision_id = $1 and name = $2
	`, revisionID, routeName).Scan(&currentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("resolve route for revision: route %s was not cloned", routeID)
	}
	if err != nil {
		return "", fmt.Errorf("resolve cloned route: %w", err)
	}
	return currentID, nil
}
