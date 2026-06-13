package observability

import (
	"context"
	"fmt"
)

// CloneSinks clones sink records into a draft revision.
func CloneSinks(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	type sinkClone struct {
		oldID string
		sink  Sink
	}
	rows, err := q.Query(ctx, `
		select id::text, name, kind, enabled, config
		from sinks
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load sinks for draft clone: %w", err)
	}
	defer rows.Close()

	var items []sinkClone
	for rows.Next() {
		var item sinkClone
		if err := rows.Scan(&item.oldID, &item.sink.Name, &item.sink.Kind, &item.sink.Enabled, &item.sink.Config); err != nil {
			return nil, fmt.Errorf("scan sink for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sinks for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		err := q.QueryRow(ctx, `
			insert into sinks (revision_id, name, kind, enabled, config)
			values ($1, $2, $3, $4, $5)
			returning id::text
		`, draftRevisionID, item.sink.Name, item.sink.Kind, item.sink.Enabled, item.sink.Config).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone sink %s: %w", item.sink.Name, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneRetentionPolicies clones retention policy records into a draft revision.
func CloneRetentionPolicies(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, sinkIDs map[string]string) error {
	type retentionClone struct {
		policy    RetentionPolicy
		oldSinkID string
	}
	rows, err := q.Query(ctx, `
		select name, coalesce(sink_id::text, ''), event_type, retention_days
		from retention_policies
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load retention policies for draft clone: %w", err)
	}
	defer rows.Close()

	var items []retentionClone
	for rows.Next() {
		var item retentionClone
		if err := rows.Scan(&item.policy.Name, &item.oldSinkID, &item.policy.EventType, &item.policy.RetentionDays); err != nil {
			return fmt.Errorf("scan retention policy for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate retention policies for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		sinkID, err := remapOptionalID(item.oldSinkID, sinkIDs, "retention policy sink")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into retention_policies (revision_id, name, sink_id, event_type, retention_days)
			values ($1, $2, nullif($3, '')::uuid, $4, $5)
		`, draftRevisionID, item.policy.Name, sinkID, item.policy.EventType, item.policy.RetentionDays); err != nil {
			return fmt.Errorf("clone retention policy %s: %w", item.policy.Name, err)
		}
	}
	return nil
}

// CloneRawCapturePolicies clones raw capture policy records into a draft revision.
func CloneRawCapturePolicies(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string, routeIDs map[string]string) error {
	type rawCaptureClone struct {
		policy     RawCapturePolicy
		oldRouteID string
	}
	rows, err := q.Query(ctx, `
		select name, coalesce(route_id::text, ''), direction, enabled, sample_rate::float8, redaction_mode
		from raw_capture_policies
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load raw capture policies for draft clone: %w", err)
	}
	defer rows.Close()

	var items []rawCaptureClone
	for rows.Next() {
		var item rawCaptureClone
		if err := rows.Scan(&item.policy.Name, &item.oldRouteID, &item.policy.Direction, &item.policy.Enabled, &item.policy.SampleRate, &item.policy.RedactionMode); err != nil {
			return fmt.Errorf("scan raw capture policy for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate raw capture policies for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		routeID, err := remapOptionalID(item.oldRouteID, routeIDs, "raw capture policy route")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into raw_capture_policies (revision_id, name, route_id, direction, enabled, sample_rate, redaction_mode)
			values ($1, $2, nullif($3, '')::uuid, $4, $5, $6, $7)
		`, draftRevisionID, item.policy.Name, routeID, item.policy.Direction, item.policy.Enabled, item.policy.SampleRate, item.policy.RedactionMode); err != nil {
			return fmt.Errorf("clone raw capture policy %s: %w", item.policy.Name, err)
		}
	}
	return nil
}

func remapOptionalID(oldID string, ids map[string]string, label string) (string, error) {
	if oldID == "" {
		return "", nil
	}
	if newID, ok := ids[oldID]; ok {
		return newID, nil
	}
	return "", fmt.Errorf("missing cloned %s id for %s", label, oldID)
}
