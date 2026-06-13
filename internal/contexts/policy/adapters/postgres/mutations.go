package postgres

import (
	"context"
	"encoding/json"
	"fmt"
)

// UpdateDraftBundleActivation marks the requested bundles active in a draft revision.
func UpdateDraftBundleActivation(ctx context.Context, q Queryer, revisionID string, keys []string) error {
	if _, err := q.Exec(ctx, `
		update policy_bundles
		set enabled = name = any($2),
			updated_at = now()
		where revision_id = $1
	`, revisionID, keys); err != nil {
		return fmt.Errorf("update policy bundle activation state: %w", err)
	}
	return nil
}

// RecordActivation records the policy activation audit row for an activated revision.
func RecordActivation(ctx context.Context, q Queryer, revisionID, snapshotID, actor, reason string) error {
	if _, err := q.Exec(ctx, `
		insert into policy_activation_records (revision_id, snapshot_id, actor, status, reason)
		values ($1, $2, $3, 'active', $4)
	`, revisionID, snapshotID, actor, reason); err != nil {
		return fmt.Errorf("record policy activation: %w", err)
	}
	return nil
}

// UpsertBundle writes a policy bundle graph into a draft revision.
func UpsertBundle(ctx context.Context, q Queryer, revisionID string, bundle Bundle) error {
	bundle.Source = defaultBundleSource(bundle.Source)
	bundle.DefaultAction = defaultBundleAction(bundle.DefaultAction)
	bundle.Metadata = defaultJSONObject(bundle.Metadata)
	var bundleID string
	err := q.QueryRow(ctx, `
		insert into policy_bundles (revision_id, name, source, version, description, default_action, enabled, metadata)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (revision_id, name) do update
		set source = excluded.source,
			version = excluded.version,
			description = excluded.description,
			default_action = excluded.default_action,
			enabled = excluded.enabled,
			metadata = excluded.metadata,
			updated_at = now()
		returning id::text
	`, revisionID, bundle.Key, bundle.Source, bundle.Version, bundle.Description, bundle.DefaultAction, bundle.Enabled, bundle.Metadata).Scan(&bundleID)
	if err != nil {
		return fmt.Errorf("upsert policy bundle: %w", err)
	}
	if _, err := q.Exec(ctx, `delete from policy_detectors where bundle_id = $1`, bundleID); err != nil {
		return fmt.Errorf("replace policy detectors: %w", err)
	}
	if _, err := q.Exec(ctx, `delete from policy_rules where bundle_id = $1`, bundleID); err != nil {
		return fmt.Errorf("replace policy rules: %w", err)
	}

	detectorIDs := make(map[string]string, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		config := detector.Config
		if config == nil {
			config = detectorConfig(detector.Kind, detector.Categories)
		}
		kind := detectorStorageKind(detector.Kind)
		var detectorID string
		err := q.QueryRow(ctx, `
			insert into policy_detectors (bundle_id, name, detector_type, pattern, config, enabled)
			values ($1, $2, $3, $4, $5, $6)
			returning id::text
		`, bundleID, detector.Key, kind, detector.Pattern, config, detector.Enabled).Scan(&detectorID)
		if err != nil {
			return fmt.Errorf("insert policy detector: %w", err)
		}
		detectorIDs[detector.Key] = detectorID
	}
	for _, rule := range bundle.Rules {
		config := rule.Config
		if config == nil {
			config = json.RawMessage(`{}`)
		}
		var ruleID string
		err := q.QueryRow(ctx, `
			insert into policy_rules (bundle_id, name, description, severity, action, enabled, priority, config)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			returning id::text
		`, bundleID, rule.Key, rule.Description, rule.Severity, rule.Action, rule.Enabled, rule.Priority, config).Scan(&ruleID)
		if err != nil {
			return fmt.Errorf("insert policy rule: %w", err)
		}
		for _, detectorKey := range rule.DetectorKeys {
			detectorID, ok := detectorIDs[detectorKey]
			if !ok {
				return fmt.Errorf("insert policy rule detector: detector %s not found", detectorKey)
			}
			if _, err := q.Exec(ctx, `insert into policy_rule_detectors (rule_id, detector_id) values ($1, $2)`, ruleID, detectorID); err != nil {
				return fmt.Errorf("insert policy rule detector: %w", err)
			}
		}
		for _, scope := range rule.Scopes {
			if _, err := q.Exec(ctx, `
				insert into policy_rule_scopes (rule_id, route_id, provider_id, model, direction)
				values ($1, nullif($2, '')::uuid, nullif($3, '')::uuid, $4, $5)
			`, ruleID, scope.RouteID, scope.ProviderID, scope.Model, storageDirection(scope.Direction)); err != nil {
				return fmt.Errorf("insert policy rule scope: %w", err)
			}
		}
	}
	return nil
}
