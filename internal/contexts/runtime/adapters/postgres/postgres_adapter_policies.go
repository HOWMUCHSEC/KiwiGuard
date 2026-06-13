package runtime

import (
	"fmt"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// convertPostgresPolicyBundles converts persisted bundles into runtime policy bundles.
func convertPostgresPolicyBundles(bundles []policystore.Bundle, routeNames map[string]string, providerNames map[string]string) []policy.Bundle {
	converted := make([]policy.Bundle, 0, len(bundles))
	for _, bundle := range bundles {
		if !bundle.Enabled {
			continue
		}
		converted = append(converted, convertPostgresPolicyBundle(bundle, routeNames, providerNames))
	}
	return converted
}

// convertPostgresPolicyBundle converts one persisted bundle into the runtime policy contract.
func convertPostgresPolicyBundle(bundle policystore.Bundle, routeNames map[string]string, providerNames map[string]string) policy.Bundle {
	detectorDefs := make([]detection.Definition, 0, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		if !detector.Enabled {
			continue
		}
		detectorDefs = append(detectorDefs, detection.Definition{
			Key:        detector.Key,
			Kind:       detection.Kind(detector.Kind),
			Pattern:    detector.Pattern,
			Categories: append([]string(nil), detector.Categories...),
		})
	}

	rules := make([]policy.Rule, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		if !rule.Enabled {
			continue
		}
		if len(rule.Scopes) == 0 {
			rules = append(rules, convertPostgresRule(rule, policy.Scope{}))
			continue
		}
		for i, scope := range rule.Scopes {
			policyScope := policy.Scope{
				RouteKey:  routeNames[scope.RouteID],
				Provider:  providerNames[scope.ProviderID],
				Model:     scope.Model,
				Direction: detection.Direction(scope.Direction),
			}
			converted := convertPostgresRule(rule, policyScope)
			if i > 0 {
				converted.Key = fmt.Sprintf("%s-scope-%d", converted.Key, i+1)
			}
			rules = append(rules, converted)
		}
	}

	defaultAction := policy.Action(bundle.DefaultAction)
	if defaultAction == "" {
		defaultAction = policy.ActionAllow
	}
	return policy.Bundle{
		Key:           bundle.Key,
		Version:       bundle.Version,
		Source:        policy.Source(bundle.Source),
		DefaultAction: defaultAction,
		Detectors:     detectorDefs,
		Rules:         rules,
	}
}

// convertPostgresRule converts one persisted rule into the runtime policy contract.
func convertPostgresRule(rule policystore.Rule, scope policy.Scope) policy.Rule {
	return policy.Rule{
		Key:          rule.Key,
		Enabled:      rule.Enabled,
		Severity:     policy.Severity(rule.Severity),
		Action:       policy.Action(rule.Action),
		DetectorKeys: append([]string(nil), rule.DetectorKeys...),
		Scope:        scope,
	}
}
