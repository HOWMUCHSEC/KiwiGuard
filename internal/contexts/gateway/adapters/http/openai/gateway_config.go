package openai

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	verdicthttp "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/adapters/httpprovider"
	domainverdict "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// BuildConfig converts application runtime configuration into HTTP gateway transport configuration.
func BuildConfig(input RuntimeConfig, opts CompileOptions, snapshot *policy.Snapshot) (Config, error) {
	verdictProviders, err := gatewayVerdictProviders(input, opts.CredentialResolver)
	if err != nil {
		return Config{}, fmt.Errorf("compile gateway runtime verdict provider: %w", err)
	}
	routeVerdictBindings, err := routeVerdictBindingsByRoute(input.RouteVerdictProviderBindings, verdictProviders.byKey)
	if err != nil {
		return Config{}, fmt.Errorf("compile gateway runtime route verdict provider bindings: %w", err)
	}
	if err := applyGlobalVerdictProviderFallback(&verdictProviders, routeVerdictBindings); err != nil {
		return Config{}, fmt.Errorf("compile gateway runtime verdict provider: %w", err)
	}
	routeLimits, err := gatewayRouteLimitPolicies(input)
	if err != nil {
		return Config{}, fmt.Errorf("compile gateway runtime route limits: %w", err)
	}
	clientLimitOverrides, err := gatewayClientRouteLimitOverrides(input, routeLimitRouteKeys(routeLimits))
	if err != nil {
		return Config{}, fmt.Errorf("compile gateway runtime client route limit overrides: %w", err)
	}
	providers, err := gatewayProviders(input, opts.CredentialResolver)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ConfigRevisionNumber:      input.Revision.Number,
		MaxBodyBytes:              opts.MaxBodyBytes,
		UpstreamTimeout:           opts.UpstreamTimeout,
		VerdictTimeout:            opts.VerdictTimeout,
		Routes:                    nil,
		Providers:                 providers,
		Snapshot:                  snapshot,
		VerdictProvider:           verdictProviders.global,
		VerdictProviders:          verdictProviders.byKey,
		EventWriter:               opts.EventWriter,
		AuditGate:                 opts.AuditGate,
		RawCapturePolicies:        gatewayRawCapturePolicies(input),
		Clients:                   gatewayClients(input),
		RouteLimits:               routeLimits,
		ClientRouteLimitOverrides: clientLimitOverrides,
	}

	routes, err := gatewayRoutes(input, routeVerdictBindings, protectedRouteKeys(input.Routes, input.GatewayClients, routeLimits))
	if err != nil {
		return Config{}, err
	}
	cfg.Routes = routes

	return cfg, nil
}

func gatewayRawCapturePolicies(input RuntimeConfig) []RawCapturePolicy {
	policies := make([]RawCapturePolicy, 0, len(input.RawCapture))
	for _, capture := range input.RawCapture {
		policies = append(policies, RawCapturePolicy{
			ID:            capture.ID,
			RouteKey:      capture.RouteKey,
			Direction:     capture.Direction,
			Enabled:       capture.Enabled,
			SampleRate:    capture.SampleRate,
			RedactionMode: capture.RedactionMode,
		})
	}
	return policies
}

func gatewayRoutes(input RuntimeConfig, verdictBindings map[string]RouteVerdictProviderBindingConfig, protectedRoutes map[string]struct{}) ([]Route, error) {
	routes := make([]Route, 0, len(input.Routes))
	mappings, err := modelMappingsByRoute(input.ModelMappings)
	if err != nil {
		return nil, err
	}
	for _, route := range input.Routes {
		if route.Disabled {
			continue
		}
		providerKey := route.ProviderKey
		mapping := ModelMapping{
			Requested: route.RequestedModel,
			Mapped:    route.MappedModel,
			Upstream:  route.UpstreamModel,
		}
		if mapped, ok := mappings[route.Key]; ok {
			mapping = mapped.mapping
			if mapped.providerKey != "" {
				providerKey = mapped.providerKey
			}
		}
		verdictProviderKey := ""
		executionMode := route.ExecutionMode
		if binding, ok := verdictBindings[route.Key]; ok {
			verdictProviderKey = binding.VerdictProviderKey
			if binding.ExecutionMode != "" {
				executionMode = binding.ExecutionMode
			}
		}
		routes = append(routes, Route{
			Key:                route.Key,
			Method:             route.Method,
			Path:               route.Path,
			ProviderKey:        providerKey,
			VerdictProviderKey: verdictProviderKey,
			ModelMapping:       mapping,
			Execution:          ExecutionMode(executionMode),
			Fallback:           Action(route.FallbackAction),
			RequireClientAuth:  hasKey(protectedRoutes, route.Key),
		})
	}
	return routes, nil
}

func gatewayClients(input RuntimeConfig) []Client {
	clients := make([]Client, 0, len(input.GatewayClients))
	for _, client := range input.GatewayClients {
		clients = append(clients, Client{
			ID:        client.ID,
			Name:      client.Name,
			Status:    ClientStatus(client.Status),
			KeyPrefix: client.KeyPrefix,
			KeyHash:   client.KeyHash,
		})
	}
	return clients
}

func gatewayRouteLimitPolicies(input RuntimeConfig) ([]RouteLimitPolicy, error) {
	routeKeys := enabledRouteKeys(input.Routes)
	policies := make([]RouteLimitPolicy, 0, len(input.RouteLimits))
	for _, record := range input.RouteLimits {
		if record.Disabled {
			continue
		}
		if !hasKey(routeKeys, record.RouteKey) {
			return nil, fmt.Errorf("route limit references unknown route %q", record.RouteKey)
		}
		if !validLimitValues(record.RequestsPerWindow, record.Window, record.MaxConcurrentRequests, record.MaxBodyBytes) {
			return nil, fmt.Errorf("route limit for route %q: limit values must be greater than zero", record.RouteKey)
		}
		policies = append(policies, RouteLimitPolicy{
			RouteKey:              record.RouteKey,
			RequestsPerWindow:     record.RequestsPerWindow,
			Window:                record.Window,
			MaxConcurrentRequests: record.MaxConcurrentRequests,
			MaxBodyBytes:          record.MaxBodyBytes,
			Enabled:               true,
		})
	}
	return policies, nil
}

func gatewayClientRouteLimitOverrides(input RuntimeConfig, routeLimitRoutes map[string]struct{}) ([]ClientRouteLimitOverride, error) {
	routeKeys := enabledRouteKeys(input.Routes)
	clientIDs := knownClientIDs(input.GatewayClients)
	overrides := make([]ClientRouteLimitOverride, 0, len(input.ClientRouteLimitOverrides))
	for _, record := range input.ClientRouteLimitOverrides {
		if record.Disabled {
			continue
		}
		if !hasKey(routeKeys, record.RouteKey) {
			return nil, fmt.Errorf("client route limit override references unknown route %q", record.RouteKey)
		}
		if !hasKey(clientIDs, record.ClientID) {
			return nil, fmt.Errorf("client route limit override references unknown client %q", record.ClientID)
		}
		if !hasKey(routeLimitRoutes, record.RouteKey) {
			return nil, fmt.Errorf("client route limit override references route %q without an enabled route limit", record.RouteKey)
		}
		if !validLimitValues(record.RequestsPerWindow, record.Window, record.MaxConcurrentRequests, record.MaxBodyBytes) {
			return nil, fmt.Errorf("client route limit override for client %q route %q: limit values must be greater than zero", record.ClientID, record.RouteKey)
		}
		overrides = append(overrides, ClientRouteLimitOverride{
			ClientID:              record.ClientID,
			RouteKey:              record.RouteKey,
			RequestsPerWindow:     record.RequestsPerWindow,
			Window:                record.Window,
			MaxConcurrentRequests: record.MaxConcurrentRequests,
			MaxBodyBytes:          record.MaxBodyBytes,
			Enabled:               true,
		})
	}
	return overrides, nil
}

func routeLimitRouteKeys(routeLimits []RouteLimitPolicy) map[string]struct{} {
	keys := make(map[string]struct{}, len(routeLimits))
	for _, policy := range routeLimits {
		keys[policy.RouteKey] = struct{}{}
	}
	return keys
}

func enabledRouteKeys(routes []RouteConfig) map[string]struct{} {
	keys := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		if route.Disabled || route.Key == "" {
			continue
		}
		keys[route.Key] = struct{}{}
	}
	return keys
}

func knownClientIDs(clients []GatewayClientConfig) map[string]struct{} {
	ids := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if client.ID == "" {
			continue
		}
		ids[client.ID] = struct{}{}
	}
	return ids
}

func protectedRouteKeys(routes []RouteConfig, clients []GatewayClientConfig, routeLimits []RouteLimitPolicy) map[string]struct{} {
	keys := make(map[string]struct{}, len(routeLimits))
	for _, policy := range routeLimits {
		keys[policy.RouteKey] = struct{}{}
	}
	if len(clients) > 0 {
		for _, route := range routes {
			if route.Disabled || route.Key == "" {
				continue
			}
			keys[route.Key] = struct{}{}
		}
	}
	return keys
}

func hasKey(keys map[string]struct{}, key string) bool {
	_, ok := keys[key]
	return ok
}

func validLimitValues(requestsPerWindow int, window time.Duration, maxConcurrentRequests int, maxBodyBytes int64) bool {
	return requestsPerWindow > 0 && window > 0 && maxConcurrentRequests > 0 && maxBodyBytes > 0
}

type routeModelMapping struct {
	key         string
	providerKey string
	mapping     ModelMapping
}

func modelMappingsByRoute(records []ModelMappingConfig) (map[string]routeModelMapping, error) {
	mappings := make(map[string]routeModelMapping, len(records))
	for _, record := range records {
		if record.Disabled || record.RouteKey == "" {
			continue
		}
		if existing, ok := mappings[record.RouteKey]; ok {
			return nil, fmt.Errorf("compile gateway runtime: duplicate active model mappings for route %q: %q and %q", record.RouteKey, existing.key, record.Key)
		}
		mappings[record.RouteKey] = routeModelMapping{
			key:         record.Key,
			providerKey: record.ProviderKey,
			mapping: ModelMapping{
				Requested: record.RequestedModel,
				Mapped:    record.MappedModel,
				Upstream:  record.UpstreamModel,
			},
		}
	}
	return mappings, nil
}

func gatewayProviders(input RuntimeConfig, resolver CredentialResolver) ([]Provider, error) {
	providers := make([]Provider, 0, len(input.Providers))
	for _, provider := range input.Providers {
		if provider.Disabled {
			continue
		}
		apiKey, err := providerAPIKey(provider, resolver)
		if err != nil {
			return nil, err
		}
		providers = append(providers, Provider{
			Key:     provider.Key,
			BaseURL: provider.BaseURL,
			APIKey:  apiKey,
			Headers: cloneStringMap(provider.Headers),
			Timeout: provider.Timeout,
		})
	}
	return providers, nil
}

func providerAPIKey(provider ProviderConfig, resolver CredentialResolver) (string, error) {
	if provider.CredentialRef == "" {
		return provider.APIKey, nil
	}
	if resolver == nil {
		return "", fmt.Errorf("compile gateway runtime provider %q credential: %w", provider.Key, ErrCredentialNotFound)
	}
	value, err := resolver.ResolveCredential(provider.CredentialRef)
	if err != nil {
		return "", fmt.Errorf("compile gateway runtime provider %q credential: %w", provider.Key, err)
	}
	if value == "" {
		return "", fmt.Errorf("compile gateway runtime provider %q credential: %w", provider.Key, ErrCredentialNotFound)
	}
	return value, nil
}

type compiledVerdictProviders struct {
	global domainverdict.Provider
	byKey  map[string]domainverdict.Provider
}

func gatewayVerdictProviders(input RuntimeConfig, resolver CredentialResolver) (compiledVerdictProviders, error) {
	compiled := compiledVerdictProviders{byKey: map[string]domainverdict.Provider{}}
	for i := range input.VerdictProviders {
		provider := &input.VerdictProviders[i]
		if !provider.Enabled || provider.Endpoint == "" {
			continue
		}
		key := verdictProviderKey(*provider)
		if key == "" {
			return compiledVerdictProviders{}, fmt.Errorf("enabled verdict provider %q is missing a key", provider.Name)
		}
		if _, exists := compiled.byKey[key]; exists {
			return compiledVerdictProviders{}, fmt.Errorf("duplicate enabled verdict provider key %q", key)
		}
		apiKey, err := verdictProviderAPIKey(*provider, resolver)
		if err != nil {
			return compiledVerdictProviders{}, err
		}
		compiled.byKey[key] = verdicthttp.NewHTTPProvider(verdicthttp.HTTPProviderOptions{
			Name:       provider.Name,
			Endpoint:   provider.Endpoint,
			APIKey:     apiKey,
			HTTPClient: http.DefaultClient,
			Timeout:    provider.Timeout,
		})
	}
	if len(compiled.byKey) == 0 {
		compiled.byKey = nil
	}
	return compiled, nil
}

func verdictProviderAPIKey(provider VerdictProviderConfig, resolver CredentialResolver) (string, error) {
	if provider.CredentialRef == "" {
		return "", nil
	}
	if resolver == nil {
		return "", fmt.Errorf("compile gateway runtime verdict provider %q credential: %w", verdictProviderKey(provider), ErrCredentialNotFound)
	}
	value, err := resolver.ResolveCredential(provider.CredentialRef)
	if err != nil {
		return "", fmt.Errorf("compile gateway runtime verdict provider %q credential: %w", verdictProviderKey(provider), err)
	}
	if value == "" {
		return "", fmt.Errorf("compile gateway runtime verdict provider %q credential: %w", verdictProviderKey(provider), ErrCredentialNotFound)
	}
	return value, nil
}

func applyGlobalVerdictProviderFallback(compiled *compiledVerdictProviders, routeBindings map[string]RouteVerdictProviderBindingConfig) error {
	if len(routeBindings) > 0 {
		return nil
	}
	if len(compiled.byKey) > 1 {
		return fmt.Errorf("multiple enabled verdict providers with endpoints configured")
	}
	for _, provider := range compiled.byKey {
		compiled.global = provider
	}
	return nil
}

func routeVerdictBindingsByRoute(records []RouteVerdictProviderBindingConfig, providers map[string]domainverdict.Provider) (map[string]RouteVerdictProviderBindingConfig, error) {
	if len(records) == 0 {
		return nil, nil
	}
	sorted := append([]RouteVerdictProviderBindingConfig(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	bindings := make(map[string]RouteVerdictProviderBindingConfig, len(sorted))
	for _, record := range sorted {
		if record.Disabled || record.RouteKey == "" {
			continue
		}
		if _, exists := bindings[record.RouteKey]; exists {
			continue
		}
		if record.VerdictProviderKey == "" {
			return nil, fmt.Errorf("route %q verdict provider binding is missing a provider key", record.RouteKey)
		}
		if _, ok := providers[record.VerdictProviderKey]; !ok {
			return nil, fmt.Errorf("route %q references unknown verdict provider %q", record.RouteKey, record.VerdictProviderKey)
		}
		bindings[record.RouteKey] = record
	}
	if len(bindings) == 0 {
		return nil, nil
	}
	return bindings, nil
}

func verdictProviderKey(provider VerdictProviderConfig) string {
	if provider.Key != "" {
		return provider.Key
	}
	return provider.Name
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
