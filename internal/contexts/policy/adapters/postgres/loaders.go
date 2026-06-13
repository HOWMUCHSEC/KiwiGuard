package postgres

import (
	"context"
	"fmt"
)

// LoadBundles hydrates full policy bundles, including detectors and rules, for one config revision.
func LoadBundles(ctx context.Context, q Queryer, revisionID string) ([]Bundle, error) {
	rows, err := q.Query(ctx, `
		select id::text, name, source, version, description, default_action, enabled, metadata
		from policy_bundles
		where revision_id = $1
		order by name
	`, revisionID)
	if err != nil {
		return nil, fmt.Errorf("load policy bundles: %w", err)
	}
	defer rows.Close()

	var bundles []Bundle
	for rows.Next() {
		var bundle Bundle
		if err := rows.Scan(&bundle.ID, &bundle.Key, &bundle.Source, &bundle.Version, &bundle.Description, &bundle.DefaultAction, &bundle.Enabled, &bundle.Metadata); err != nil {
			return nil, fmt.Errorf("scan policy bundle: %w", err)
		}
		bundles = append(bundles, bundle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy bundles: %w", err)
	}
	rows.Close()
	if err := hydratePolicyBundles(ctx, q, bundles); err != nil {
		return nil, err
	}
	return bundles, nil
}

// LoadBundlesByKeys hydrates only the requested policy bundles for one config revision.
func LoadBundlesByKeys(ctx context.Context, q Queryer, revisionID string, keys []string) ([]Bundle, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	rows, err := q.Query(ctx, `
		select id::text, name, source, version, description, default_action, enabled, metadata
		from policy_bundles
		where revision_id = $1 and name = any($2)
		order by name
	`, revisionID, keys)
	if err != nil {
		return nil, fmt.Errorf("load policy bundles by keys: %w", err)
	}
	defer rows.Close()

	bundles := make([]Bundle, 0, len(keys))
	for rows.Next() {
		var bundle Bundle
		if err := rows.Scan(&bundle.ID, &bundle.Key, &bundle.Source, &bundle.Version, &bundle.Description, &bundle.DefaultAction, &bundle.Enabled, &bundle.Metadata); err != nil {
			return nil, fmt.Errorf("scan policy bundle by key: %w", err)
		}
		bundles = append(bundles, bundle)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy bundles by keys: %w", err)
	}
	rows.Close()
	if err := hydratePolicyBundles(ctx, q, bundles); err != nil {
		return nil, err
	}
	return bundles, nil
}

func hydratePolicyBundles(ctx context.Context, q Queryer, bundles []Bundle) error {
	if len(bundles) == 0 {
		return nil
	}
	bundleIDs := make([]string, 0, len(bundles))
	bundleIndex := make(map[string]int, len(bundles))
	for i := range bundles {
		bundleIDs = append(bundleIDs, bundles[i].ID)
		bundleIndex[bundles[i].ID] = i
	}
	if err := hydratePolicyBundleDetectors(ctx, q, bundles, bundleIDs, bundleIndex); err != nil {
		return err
	}
	return hydratePolicyBundleRules(ctx, q, bundles, bundleIDs, bundleIndex)
}

func hydratePolicyBundleDetectors(ctx context.Context, q Queryer, bundles []Bundle, bundleIDs []string, bundleIndex map[string]int) error {
	rows, err := q.Query(ctx, `
		select bundle_id::text, id::text, name, detector_type, pattern, config, enabled
		from policy_detectors
		where bundle_id = any($1)
		order by bundle_id, name
	`, bundleIDs)
	if err != nil {
		return fmt.Errorf("load policy detectors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bundleID string
		var detector Detector
		if err := rows.Scan(&bundleID, &detector.ID, &detector.Key, &detector.Kind, &detector.Pattern, &detector.Config, &detector.Enabled); err != nil {
			return fmt.Errorf("scan policy detector: %w", err)
		}
		detector.Categories = detectorCategories(detector.Config)
		detector.Kind = detectorPolicyKind(detector.Kind, detector.Config)
		if i, ok := bundleIndex[bundleID]; ok {
			bundles[i].Detectors = append(bundles[i].Detectors, detector)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy detectors: %w", err)
	}
	return nil
}

func hydratePolicyBundleRules(ctx context.Context, q Queryer, bundles []Bundle, bundleIDs []string, bundleIndex map[string]int) error {
	rows, err := q.Query(ctx, `
		select bundle_id::text, id::text, name, description, severity, action, enabled, priority, config
		from policy_rules
		where bundle_id = any($1)
		order by bundle_id, priority, name
	`, bundleIDs)
	if err != nil {
		return fmt.Errorf("load policy rules: %w", err)
	}
	defer rows.Close()

	ruleBundle := map[string]string{}
	ruleIndex := map[string]int{}
	ruleIDs := []string{}
	for rows.Next() {
		var bundleID string
		var rule Rule
		if err := rows.Scan(&bundleID, &rule.ID, &rule.Key, &rule.Description, &rule.Severity, &rule.Action, &rule.Enabled, &rule.Priority, &rule.Config); err != nil {
			return fmt.Errorf("scan policy rule: %w", err)
		}
		i, ok := bundleIndex[bundleID]
		if !ok {
			continue
		}
		bundles[i].Rules = append(bundles[i].Rules, rule)
		ruleBundle[rule.ID] = bundleID
		ruleIndex[rule.ID] = len(bundles[i].Rules) - 1
		ruleIDs = append(ruleIDs, rule.ID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy rules: %w", err)
	}
	rows.Close()
	if len(ruleIDs) == 0 {
		return nil
	}
	if err := hydratePolicyRuleDetectorKeys(ctx, q, bundles, bundleIndex, ruleBundle, ruleIndex, ruleIDs); err != nil {
		return err
	}
	return hydratePolicyRuleScopes(ctx, q, bundles, bundleIndex, ruleBundle, ruleIndex, ruleIDs)
}

func hydratePolicyRuleDetectorKeys(ctx context.Context, q Queryer, bundles []Bundle, bundleIndex map[string]int, ruleBundle map[string]string, ruleIndex map[string]int, ruleIDs []string) error {
	rows, err := q.Query(ctx, `
		select rd.rule_id::text, d.name
		from policy_rule_detectors rd
		join policy_detectors d on d.id = rd.detector_id
		where rd.rule_id = any($1)
		order by rd.rule_id, d.name
	`, ruleIDs)
	if err != nil {
		return fmt.Errorf("load policy rule detectors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ruleID string
		var detectorKey string
		if err := rows.Scan(&ruleID, &detectorKey); err != nil {
			return fmt.Errorf("scan policy rule detector: %w", err)
		}
		bundleID := ruleBundle[ruleID]
		bundles[bundleIndex[bundleID]].Rules[ruleIndex[ruleID]].DetectorKeys = append(bundles[bundleIndex[bundleID]].Rules[ruleIndex[ruleID]].DetectorKeys, detectorKey)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy rule detectors: %w", err)
	}
	return nil
}

func hydratePolicyRuleScopes(ctx context.Context, q Queryer, bundles []Bundle, bundleIndex map[string]int, ruleBundle map[string]string, ruleIndex map[string]int, ruleIDs []string) error {
	rows, err := q.Query(ctx, `
		select rule_id::text, coalesce(route_id::text, ''), coalesce(provider_id::text, ''), model, direction
		from policy_rule_scopes
		where rule_id = any($1)
		order by rule_id, model, direction
	`, ruleIDs)
	if err != nil {
		return fmt.Errorf("load policy rule scopes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ruleID string
		var scope RuleScope
		if err := rows.Scan(&ruleID, &scope.RouteID, &scope.ProviderID, &scope.Model, &scope.Direction); err != nil {
			return fmt.Errorf("scan policy rule scope: %w", err)
		}
		scope.Direction = policyDirection(scope.Direction)
		bundleID := ruleBundle[ruleID]
		bundles[bundleIndex[bundleID]].Rules[ruleIndex[ruleID]].Scopes = append(bundles[bundleIndex[bundleID]].Rules[ruleIndex[ruleID]].Scopes, scope)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate policy rule scopes: %w", err)
	}
	return nil
}
