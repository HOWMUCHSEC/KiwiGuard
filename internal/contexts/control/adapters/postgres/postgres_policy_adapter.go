package postgres

import (
	"context"
	"errors"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// ListPolicyBundles projects the currently visible persisted bundles into control-plane contracts.
func (s postgresPolicyStore) ListPolicyBundles(ctx context.Context) ([]appcontrol.PolicyBundle, error) {
	bundles, err := s.repo.ListPolicyBundles(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		return []appcontrol.PolicyBundle{}, nil
	}
	if err != nil {
		return nil, err
	}
	routes, err := s.repo.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}
	providers, err := s.repo.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	routeNames := routeNamesByID(routes)
	providerNames := providerNamesByID(providers)

	items := make([]appcontrol.PolicyBundle, 0, len(bundles))
	for _, bundle := range bundles {
		items = append(items, policyBundleFromPostgres(bundle, routeNames, providerNames))
	}
	return items, nil
}

// CreatePolicyBundle validates dependencies and upserts one policy bundle into draft storage.
func (s postgresPolicyStore) CreatePolicyBundle(ctx context.Context, bundle appcontrol.PolicyBundle) error {
	routes, err := s.repo.ListRoutes(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		routes = nil
	} else if err != nil {
		return err
	}
	providers, err := s.repo.ListProviders(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		providers = nil
	} else if err != nil {
		return err
	}
	postgresBundle, err := policyBundleToPostgres(bundle, routeIDsByName(routes), providerIDsByName(providers))
	if err != nil {
		return err
	}
	return s.repo.UpsertPolicyBundle(ctx, postgresBundle)
}

// ActivatePolicyBundles promotes the requested bundle set to the active revision.
func (s postgresPolicyStore) ActivatePolicyBundles(ctx context.Context, request appcontrol.PolicyActivationRequest) (appcontrol.PolicyActivationResponse, error) {
	result, err := s.repo.ActivatePolicyBundles(ctx, policystore.ActivationRequest{
		Keys:         append([]string(nil), request.Keys...),
		Actor:        "control-api",
		Reason:       request.Reason,
		SnapshotHash: request.SnapshotHash,
	})
	if err != nil {
		return appcontrol.PolicyActivationResponse{}, err
	}
	return appcontrol.PolicyActivationResponse{
		ActiveKeys:     append([]string(nil), result.ActiveKeys...),
		Hash:           result.SnapshotHash,
		RevisionNumber: result.RevisionNumber,
	}, nil
}

func policyBundleFromPostgres(bundle policystore.Bundle, routeNames map[string]string, providerNames map[string]string) appcontrol.PolicyBundle {
	detectorsOut := make([]appcontrol.Detector, 0, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		detectorsOut = append(detectorsOut, appcontrol.Detector{
			Key:        detector.Key,
			Kind:       detector.Kind,
			Pattern:    detector.Pattern,
			Categories: append([]string(nil), detector.Categories...),
		})
	}
	rules := make([]appcontrol.Rule, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		scope := appcontrol.Scope{}
		if len(rule.Scopes) > 0 {
			scope.RouteKey = routeNames[rule.Scopes[0].RouteID]
			scope.Provider = providerNames[rule.Scopes[0].ProviderID]
			scope.Model = rule.Scopes[0].Model
			scope.Direction = rule.Scopes[0].Direction
		}
		rules = append(rules, appcontrol.Rule{
			Key:          rule.Key,
			Enabled:      rule.Enabled,
			Severity:     rule.Severity,
			Action:       rule.Action,
			DetectorKeys: append([]string(nil), rule.DetectorKeys...),
			Scope:        scope,
		})
	}
	defaultAction := bundle.DefaultAction
	if defaultAction == "" {
		defaultAction = string(policy.ActionAllow)
	}
	return appcontrol.PolicyBundle{
		Key:           bundle.Key,
		Version:       bundle.Version,
		Source:        bundle.Source,
		DefaultAction: defaultAction,
		Detectors:     detectorsOut,
		Rules:         rules,
	}
}

func policyBundleToPostgres(bundle appcontrol.PolicyBundle, routeIDs map[string]string, providerIDs map[string]string) (policystore.Bundle, error) {
	detectorsOut := make([]policystore.Detector, 0, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		detectorsOut = append(detectorsOut, policystore.Detector{
			Key:        detector.Key,
			Kind:       detector.Kind,
			Pattern:    detector.Pattern,
			Categories: append([]string(nil), detector.Categories...),
			Enabled:    true,
		})
	}
	rules := make([]policystore.Rule, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		scopes := []policystore.RuleScope{}
		if rule.Scope.RouteKey != "" || rule.Scope.Provider != "" || rule.Scope.Direction != "" || rule.Scope.Model != "" {
			routeID, err := optionalIDByName(rule.Scope.RouteKey, routeIDs, "route")
			if err != nil {
				return policystore.Bundle{}, err
			}
			providerID, err := optionalIDByName(rule.Scope.Provider, providerIDs, "provider")
			if err != nil {
				return policystore.Bundle{}, err
			}
			scopes = append(scopes, policystore.RuleScope{
				RouteID:    routeID,
				ProviderID: providerID,
				Model:      rule.Scope.Model,
				Direction:  rule.Scope.Direction,
			})
		}
		rules = append(rules, policystore.Rule{
			Key:          rule.Key,
			Enabled:      rule.Enabled,
			Severity:     rule.Severity,
			Action:       rule.Action,
			DetectorKeys: append([]string(nil), rule.DetectorKeys...),
			Scopes:       scopes,
		})
	}
	defaultAction := bundle.DefaultAction
	if defaultAction == "" {
		defaultAction = string(policy.ActionAllow)
	}
	return policystore.Bundle{
		Key:           bundle.Key,
		Version:       bundle.Version,
		Source:        bundle.Source,
		DefaultAction: defaultAction,
		Enabled:       true,
		Detectors:     detectorsOut,
		Rules:         rules,
	}, nil
}
