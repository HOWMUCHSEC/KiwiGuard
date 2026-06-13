package observability

import (
	"context"
	"fmt"
)

// LoadSinks hydrates event-sink records attached to one config revision.
func LoadSinks(ctx context.Context, q Queryer, revisionID string) ([]Sink, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, kind, enabled, config
		from sinks
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load sinks: %w", err)
	}
	defer rows.Close()

	var sinks []Sink
	for rows.Next() {
		var sink Sink
		if err := rows.Scan(&sink.ID, &sink.Name, &sink.Kind, &sink.Enabled, &sink.Config); err != nil {
			return nil, fmt.Errorf("scan sink: %w", err)
		}
		sinks = append(sinks, sink)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sinks: %w", err)
	}
	return sinks, nil
}

// LoadRetentionPolicies hydrates retention-policy records attached to one config revision.
func LoadRetentionPolicies(ctx context.Context, q Queryer, revisionID string) ([]RetentionPolicy, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, coalesce(sink_id::text, ''), event_type, retention_days
		from retention_policies
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load retention policies: %w", err)
	}
	defer rows.Close()

	var policies []RetentionPolicy
	for rows.Next() {
		var retention RetentionPolicy
		if err := rows.Scan(&retention.ID, &retention.Name, &retention.SinkID, &retention.EventType, &retention.RetentionDays); err != nil {
			return nil, fmt.Errorf("scan retention policy: %w", err)
		}
		policies = append(policies, retention)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention policies: %w", err)
	}
	return policies, nil
}

// LoadRawCapturePolicies hydrates raw-capture policy records attached to one config revision.
func LoadRawCapturePolicies(ctx context.Context, q Queryer, revisionID string) ([]RawCapturePolicy, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, coalesce(route_id::text, ''), direction, enabled, sample_rate::float8, redaction_mode
		from raw_capture_policies
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load raw capture policies: %w", err)
	}
	defer rows.Close()

	var policies []RawCapturePolicy
	for rows.Next() {
		var capture RawCapturePolicy
		if err := rows.Scan(&capture.ID, &capture.Name, &capture.RouteID, &capture.Direction, &capture.Enabled, &capture.SampleRate, &capture.RedactionMode); err != nil {
			return nil, fmt.Errorf("scan raw capture policy: %w", err)
		}
		policies = append(policies, capture)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw capture policies: %w", err)
	}
	return policies, nil
}
