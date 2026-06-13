package httpapi

import appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"

// configStatusFromApp converts application config status into the control API response shape.
func configStatusFromApp(status appcontrol.ConfigStatus) configStatusResponse {
	return configStatusResponse{
		ActivePolicyBundleKeys: status.ActivePolicyBundleKeys,
		PolicySnapshotHash:     status.PolicySnapshotHash,
	}
}

// policyBundleToApp converts an HTTP DTO into the application policy bundle contract.
func policyBundleToApp(bundle policyBundleDTO) appcontrol.PolicyBundle {
	detectors := make([]appcontrol.Detector, 0, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		detectors = append(detectors, appcontrol.Detector{
			Key:        detector.Key,
			Kind:       detector.Kind,
			Pattern:    detector.Pattern,
			Categories: append([]string(nil), detector.Categories...),
		})
	}
	rules := make([]appcontrol.Rule, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		rules = append(rules, appcontrol.Rule{
			Key:          rule.Key,
			Enabled:      rule.Enabled,
			Severity:     rule.Severity,
			Action:       rule.Action,
			DetectorKeys: append([]string(nil), rule.DetectorKeys...),
			Scope: appcontrol.Scope{
				RouteKey:  rule.Scope.RouteKey,
				Provider:  rule.Scope.Provider,
				Model:     rule.Scope.Model,
				Direction: rule.Scope.Direction,
			},
		})
	}
	return appcontrol.PolicyBundle{
		Key:           bundle.Key,
		Version:       bundle.Version,
		Source:        bundle.Source,
		DefaultAction: bundle.DefaultAction,
		Detectors:     detectors,
		Rules:         rules,
	}
}

// policyBundleFromApp converts an application policy bundle into the HTTP DTO shape.
func policyBundleFromApp(bundle appcontrol.PolicyBundle) policyBundleDTO {
	detectors := make([]detectorDTO, 0, len(bundle.Detectors))
	for _, detector := range bundle.Detectors {
		detectors = append(detectors, detectorDTO{
			Key:        detector.Key,
			Kind:       detector.Kind,
			Pattern:    detector.Pattern,
			Categories: append([]string(nil), detector.Categories...),
		})
	}
	rules := make([]ruleDTO, 0, len(bundle.Rules))
	for _, rule := range bundle.Rules {
		rules = append(rules, ruleDTO{
			Key:          rule.Key,
			Enabled:      rule.Enabled,
			Severity:     rule.Severity,
			Action:       rule.Action,
			DetectorKeys: append([]string(nil), rule.DetectorKeys...),
			Scope: scopeDTO{
				RouteKey:  rule.Scope.RouteKey,
				Provider:  rule.Scope.Provider,
				Model:     rule.Scope.Model,
				Direction: rule.Scope.Direction,
			},
		})
	}
	return policyBundleDTO{
		Key:           bundle.Key,
		Version:       bundle.Version,
		Source:        bundle.Source,
		DefaultAction: bundle.DefaultAction,
		Detectors:     detectors,
		Rules:         rules,
	}
}

func policyBundlesFromApp(bundles []appcontrol.PolicyBundle) []policyBundleDTO {
	items := make([]policyBundleDTO, 0, len(bundles))
	for _, bundle := range bundles {
		items = append(items, policyBundleFromApp(bundle))
	}
	return items
}

func policyBundlesToApp(bundles []policyBundleDTO) []appcontrol.PolicyBundle {
	items := make([]appcontrol.PolicyBundle, 0, len(bundles))
	for _, bundle := range bundles {
		items = append(items, policyBundleToApp(bundle))
	}
	return items
}

func policyActivationRequestToApp(request policyActivationRequest) appcontrol.PolicyActivationRequest {
	return appcontrol.PolicyActivationRequest{Keys: append([]string(nil), request.Keys...), Reason: request.Reason}
}

func policyActivationResponseFromApp(response appcontrol.PolicyActivationResponse) policyActivationResponse {
	return policyActivationResponse{
		ActiveKeys:        append([]string(nil), response.ActiveKeys...),
		Hash:              response.Hash,
		NotificationError: response.NotificationError,
		RevisionNumber:    response.RevisionNumber,
	}
}

func policyDryRunRequestToApp(request policyDryRunRequest) appcontrol.PolicyDryRunRequest {
	return appcontrol.PolicyDryRunRequest{
		RouteKey:  request.RouteKey,
		Provider:  request.Provider,
		Model:     request.Model,
		Direction: request.Direction,
		Text:      request.Text,
		Bundle:    policyBundleToApp(request.Bundle),
	}
}

func modelMappingToApp(mapping modelMappingDTO) appcontrol.ModelMapping {
	return appcontrol.ModelMapping(mapping)
}

func modelMappingFromApp(mapping appcontrol.ModelMapping) modelMappingDTO {
	return modelMappingDTO(mapping)
}

func modelMappingsFromApp(mappings []appcontrol.ModelMapping) []modelMappingDTO {
	items := make([]modelMappingDTO, 0, len(mappings))
	for _, mapping := range mappings {
		items = append(items, modelMappingFromApp(mapping))
	}
	return items
}

func verdictProviderToApp(provider verdictProviderDTO) appcontrol.VerdictProvider {
	return appcontrol.VerdictProvider(provider)
}

func verdictProviderFromApp(provider appcontrol.VerdictProvider) verdictProviderDTO {
	return verdictProviderDTO(provider)
}

func verdictProvidersFromApp(providers []appcontrol.VerdictProvider) []verdictProviderDTO {
	items := make([]verdictProviderDTO, 0, len(providers))
	for _, provider := range providers {
		items = append(items, verdictProviderFromApp(provider))
	}
	return items
}

func gatewayClientToApp(client gatewayClientDTO) appcontrol.GatewayClient {
	return appcontrol.GatewayClient(client)
}

func gatewayClientFromApp(client appcontrol.GatewayClient) gatewayClientDTO {
	return gatewayClientDTO(client)
}

func gatewayClientsFromApp(clients []appcontrol.GatewayClient) []gatewayClientDTO {
	items := make([]gatewayClientDTO, 0, len(clients))
	for _, client := range clients {
		items = append(items, gatewayClientFromApp(client))
	}
	return items
}

func createGatewayClientRequestToApp(request createGatewayClientRequest) appcontrol.CreateGatewayClientRequest {
	return appcontrol.CreateGatewayClientRequest(request)
}

func createGatewayClientResponseFromApp(response appcontrol.CreateGatewayClientResponse) createGatewayClientResponse {
	return createGatewayClientResponse{
		Client: gatewayClientFromApp(response.Client),
		Key:    response.Key,
	}
}

func routeLimitToApp(limit routeLimitDTO) appcontrol.RouteLimit {
	return appcontrol.RouteLimit(limit)
}

func routeLimitFromApp(limit appcontrol.RouteLimit) routeLimitDTO {
	return routeLimitDTO(limit)
}

func routeLimitsFromApp(limits []appcontrol.RouteLimit) []routeLimitDTO {
	items := make([]routeLimitDTO, 0, len(limits))
	for _, limit := range limits {
		items = append(items, routeLimitFromApp(limit))
	}
	return items
}

func clientRouteLimitToApp(limit clientRouteLimitDTO) appcontrol.ClientRouteLimit {
	return appcontrol.ClientRouteLimit(limit)
}

func clientRouteLimitFromApp(limit appcontrol.ClientRouteLimit) clientRouteLimitDTO {
	return clientRouteLimitDTO(limit)
}

func clientRouteLimitsFromApp(limits []appcontrol.ClientRouteLimit) []clientRouteLimitDTO {
	items := make([]clientRouteLimitDTO, 0, len(limits))
	for _, limit := range limits {
		items = append(items, clientRouteLimitFromApp(limit))
	}
	return items
}
