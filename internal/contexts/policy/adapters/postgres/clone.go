package postgres

import (
	"context"
	"fmt"
)

// CloneGraph clones policy bundles, detectors, and rules into a draft revision.
func CloneGraph(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string) (map[string]string, map[string]string, map[string]string, error) {
	bundleIDs, err := clonePolicyBundles(ctx, q, sourceRevisionID, draftRevisionID)
	if err != nil {
		return nil, nil, nil, err
	}
	detectorIDs, err := clonePolicyDetectors(ctx, q, sourceRevisionID, bundleIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	ruleIDs, err := clonePolicyRules(ctx, q, sourceRevisionID, bundleIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	return bundleIDs, detectorIDs, ruleIDs, nil
}

func clonePolicyBundles(ctx context.Context, q Queryer, sourceRevisionID, draftRevisionID string) (map[string]string, error) {
	type bundleClone struct {
		oldID  string
		bundle Bundle
	}
	rows, err := q.Query(ctx, `
		select id::text, name, source, version, description, default_action, enabled, metadata
		from policy_bundles
		where revision_id = $1
		order by name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load policy bundles for draft clone: %w", err)
	}
	defer rows.Close()

	var items []bundleClone
	for rows.Next() {
		var item bundleClone
		if err := rows.Scan(&item.oldID, &item.bundle.Key, &item.bundle.Source, &item.bundle.Version, &item.bundle.Description, &item.bundle.DefaultAction, &item.bundle.Enabled, &item.bundle.Metadata); err != nil {
			return nil, fmt.Errorf("scan policy bundle for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy bundles for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		err := q.QueryRow(ctx, `
			insert into policy_bundles (revision_id, name, source, version, description, default_action, enabled, metadata)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			returning id::text
		`, draftRevisionID, item.bundle.Key, item.bundle.Source, item.bundle.Version, item.bundle.Description, item.bundle.DefaultAction, item.bundle.Enabled, item.bundle.Metadata).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone policy bundle %s: %w", item.bundle.Key, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

func clonePolicyDetectors(ctx context.Context, q Queryer, sourceRevisionID string, bundleIDs map[string]string) (map[string]string, error) {
	type detectorClone struct {
		oldID       string
		oldBundleID string
		detector    Detector
	}
	rows, err := q.Query(ctx, `
		select d.id::text, b.id::text, d.name, d.detector_type, d.pattern, d.config, d.enabled
		from policy_detectors d
		join policy_bundles b on b.id = d.bundle_id
		where b.revision_id = $1
		order by b.name, d.name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load policy detectors for draft clone: %w", err)
	}
	defer rows.Close()

	var items []detectorClone
	for rows.Next() {
		var item detectorClone
		if err := rows.Scan(&item.oldID, &item.oldBundleID, &item.detector.Key, &item.detector.Kind, &item.detector.Pattern, &item.detector.Config, &item.detector.Enabled); err != nil {
			return nil, fmt.Errorf("scan policy detector for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy detectors for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		bundleID, err := remapRequiredID(item.oldBundleID, bundleIDs, "policy detector bundle")
		if err != nil {
			return nil, err
		}
		err = q.QueryRow(ctx, `
			insert into policy_detectors (bundle_id, name, detector_type, pattern, config, enabled)
			values ($1, $2, $3, $4, $5, $6)
			returning id::text
		`, bundleID, item.detector.Key, item.detector.Kind, item.detector.Pattern, item.detector.Config, item.detector.Enabled).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone policy detector %s: %w", item.detector.Key, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

func clonePolicyRules(ctx context.Context, q Queryer, sourceRevisionID string, bundleIDs map[string]string) (map[string]string, error) {
	type ruleClone struct {
		oldID       string
		oldBundleID string
		rule        Rule
	}
	rows, err := q.Query(ctx, `
		select r.id::text, b.id::text, r.name, r.description, r.severity, r.action,
			r.enabled, r.priority, r.config
		from policy_rules r
		join policy_bundles b on b.id = r.bundle_id
		where b.revision_id = $1
		order by b.name, r.priority, r.name
	`, sourceRevisionID)
	if err != nil {
		return nil, fmt.Errorf("load policy rules for draft clone: %w", err)
	}
	defer rows.Close()

	var items []ruleClone
	for rows.Next() {
		var item ruleClone
		if err := rows.Scan(&item.oldID, &item.oldBundleID, &item.rule.Key, &item.rule.Description, &item.rule.Severity, &item.rule.Action, &item.rule.Enabled, &item.rule.Priority, &item.rule.Config); err != nil {
			return nil, fmt.Errorf("scan policy rule for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy rules for draft clone: %w", err)
	}
	rows.Close()

	ids := map[string]string{}
	for _, item := range items {
		var newID string
		bundleID, err := remapRequiredID(item.oldBundleID, bundleIDs, "policy rule bundle")
		if err != nil {
			return nil, err
		}
		err = q.QueryRow(ctx, `
			insert into policy_rules (bundle_id, name, description, severity, action, enabled, priority, config)
			values ($1, $2, $3, $4, $5, $6, $7, $8)
			returning id::text
		`, bundleID, item.rule.Key, item.rule.Description, item.rule.Severity, item.rule.Action, item.rule.Enabled, item.rule.Priority, item.rule.Config).Scan(&newID)
		if err != nil {
			return nil, fmt.Errorf("clone policy rule %s: %w", item.rule.Key, err)
		}
		ids[item.oldID] = newID
	}
	return ids, nil
}

// CloneBindings clones policy-owned bindings after policy and routing records have been cloned.
func CloneBindings(ctx context.Context, q Queryer, sourceRevisionID string, ruleIDs, detectorIDs, routeIDs, providerIDs, bundleIDs map[string]string) error {
	if err := clonePolicyRuleDetectors(ctx, q, sourceRevisionID, ruleIDs, detectorIDs); err != nil {
		return err
	}
	if err := clonePolicyRuleScopes(ctx, q, sourceRevisionID, ruleIDs, routeIDs, providerIDs); err != nil {
		return err
	}
	return cloneRoutePolicyBindings(ctx, q, sourceRevisionID, routeIDs, bundleIDs)
}

func clonePolicyRuleDetectors(ctx context.Context, q Queryer, sourceRevisionID string, ruleIDs, detectorIDs map[string]string) error {
	type ruleDetectorClone struct {
		oldRuleID     string
		oldDetectorID string
		matchMode     string
	}
	rows, err := q.Query(ctx, `
		select rd.rule_id::text, rd.detector_id::text, rd.match_mode
		from policy_rule_detectors rd
		join policy_rules r on r.id = rd.rule_id
		join policy_bundles b on b.id = r.bundle_id
		where b.revision_id = $1
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load policy rule detectors for draft clone: %w", err)
	}
	defer rows.Close()

	var items []ruleDetectorClone
	for rows.Next() {
		var item ruleDetectorClone
		if err := rows.Scan(&item.oldRuleID, &item.oldDetectorID, &item.matchMode); err != nil {
			return fmt.Errorf("scan policy rule detector for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy rule detectors for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		ruleID, err := remapRequiredID(item.oldRuleID, ruleIDs, "policy rule detector rule")
		if err != nil {
			return err
		}
		detectorID, err := remapRequiredID(item.oldDetectorID, detectorIDs, "policy rule detector detector")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `insert into policy_rule_detectors (rule_id, detector_id, match_mode) values ($1, $2, $3)`, ruleID, detectorID, item.matchMode); err != nil {
			return fmt.Errorf("clone policy rule detector: %w", err)
		}
	}
	return nil
}

func clonePolicyRuleScopes(ctx context.Context, q Queryer, sourceRevisionID string, ruleIDs, routeIDs, providerIDs map[string]string) error {
	type ruleScopeClone struct {
		oldRuleID     string
		oldRouteID    string
		oldProviderID string
		model         string
		direction     string
	}
	rows, err := q.Query(ctx, `
		select s.rule_id::text, coalesce(s.route_id::text, ''), coalesce(s.provider_id::text, ''),
			s.model, s.direction
		from policy_rule_scopes s
		join policy_rules r on r.id = s.rule_id
		join policy_bundles b on b.id = r.bundle_id
		where b.revision_id = $1
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load policy rule scopes for draft clone: %w", err)
	}
	defer rows.Close()

	var items []ruleScopeClone
	for rows.Next() {
		var item ruleScopeClone
		if err := rows.Scan(&item.oldRuleID, &item.oldRouteID, &item.oldProviderID, &item.model, &item.direction); err != nil {
			return fmt.Errorf("scan policy rule scope for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy rule scopes for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		ruleID, err := remapRequiredID(item.oldRuleID, ruleIDs, "policy rule scope rule")
		if err != nil {
			return err
		}
		routeID, err := remapOptionalID(item.oldRouteID, routeIDs, "policy rule scope route")
		if err != nil {
			return err
		}
		providerID, err := remapOptionalID(item.oldProviderID, providerIDs, "policy rule scope provider")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `
			insert into policy_rule_scopes (rule_id, route_id, provider_id, model, direction)
			values ($1, nullif($2, '')::uuid, nullif($3, '')::uuid, $4, $5)
		`, ruleID, routeID, providerID, item.model, item.direction); err != nil {
			return fmt.Errorf("clone policy rule scope: %w", err)
		}
	}
	return nil
}

func cloneRoutePolicyBindings(ctx context.Context, q Queryer, sourceRevisionID string, routeIDs, bundleIDs map[string]string) error {
	type routePolicyBindingClone struct {
		oldRouteID  string
		oldBundleID string
		enabled     bool
		priority    int
	}
	rows, err := q.Query(ctx, `
		select b.route_id::text, b.bundle_id::text, b.enabled, b.priority
		from route_policy_bindings b
		join routes r on r.id = b.route_id
		where r.revision_id = $1
	`, sourceRevisionID)
	if err != nil {
		return fmt.Errorf("load route policy bindings for draft clone: %w", err)
	}
	defer rows.Close()

	var items []routePolicyBindingClone
	for rows.Next() {
		var item routePolicyBindingClone
		if err := rows.Scan(&item.oldRouteID, &item.oldBundleID, &item.enabled, &item.priority); err != nil {
			return fmt.Errorf("scan route policy binding for draft clone: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate route policy bindings for draft clone: %w", err)
	}
	rows.Close()

	for _, item := range items {
		routeID, err := remapRequiredID(item.oldRouteID, routeIDs, "route policy binding route")
		if err != nil {
			return err
		}
		bundleID, err := remapRequiredID(item.oldBundleID, bundleIDs, "route policy binding bundle")
		if err != nil {
			return err
		}
		if _, err := q.Exec(ctx, `insert into route_policy_bindings (route_id, bundle_id, enabled, priority) values ($1, $2, $3, $4)`, routeID, bundleID, item.enabled, item.priority); err != nil {
			return fmt.Errorf("clone route policy binding: %w", err)
		}
	}
	return nil
}
