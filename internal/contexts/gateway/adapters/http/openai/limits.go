package openai

import (
	"sync"
	"time"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// limitResolver resolves effective route limits for gateway clients.
type limitResolver struct {
	protectedRoutes map[string]struct{}
	routePolicies   map[string]RouteLimitPolicy
	overrides       map[string]RouteLimitPolicy
}

// newLimitResolver builds a resolver from compiled routes, defaults, and client overrides.
func newLimitResolver(routes []Route, routePolicies []RouteLimitPolicy, overrides []ClientRouteLimitOverride) limitResolver {
	resolver := limitResolver{
		protectedRoutes: make(map[string]struct{}),
		routePolicies:   make(map[string]RouteLimitPolicy),
		overrides:       make(map[string]RouteLimitPolicy),
	}

	for _, route := range routes {
		if route.Key != "" && route.RequireClientAuth {
			resolver.protectedRoutes[route.Key] = struct{}{}
		}
	}
	for _, policy := range routePolicies {
		if !validRouteLimitPolicy(policy) {
			continue
		}
		resolver.routePolicies[policy.RouteKey] = policy
		resolver.protectedRoutes[policy.RouteKey] = struct{}{}
	}
	for _, override := range overrides {
		if !validClientRouteLimitOverride(override) {
			continue
		}
		resolver.overrides[limitKey(override.ClientID, override.RouteKey)] = RouteLimitPolicy{
			RouteKey:              override.RouteKey,
			RequestsPerWindow:     override.RequestsPerWindow,
			Window:                override.Window,
			MaxConcurrentRequests: override.MaxConcurrentRequests,
			MaxBodyBytes:          override.MaxBodyBytes,
			Enabled:               override.Enabled,
		}
	}

	return resolver
}

func (r limitResolver) routeProtected(routeKey string) bool {
	_, ok := r.protectedRoutes[routeKey]
	return ok
}

func (r limitResolver) effectivePolicy(clientID, routeKey string) (RouteLimitPolicy, bool) {
	policy, ok := r.routePolicies[routeKey]
	if !ok {
		return RouteLimitPolicy{}, false
	}
	if policy, ok := r.overrides[limitKey(clientID, routeKey)]; ok {
		return policy, true
	}
	return policy, true
}

func (r limitResolver) EffectiveLimitPolicy(clientID, routeKey string) (appgateway.LimitPolicy, bool) {
	policy, ok := r.effectivePolicy(clientID, routeKey)
	if !ok {
		return appgateway.LimitPolicy{}, false
	}
	return appgateway.LimitPolicy{
		RequestsPerWindow:     policy.RequestsPerWindow,
		Window:                policy.Window,
		MaxConcurrentRequests: policy.MaxConcurrentRequests,
		MaxBodyBytes:          policy.MaxBodyBytes,
	}, true
}

// limitState tracks in-memory rate windows and concurrent request reservations.
type limitState struct {
	mu          sync.Mutex
	rates       map[string]rateWindow
	concurrency map[string]int
}

type rateWindow struct {
	start time.Time
	count int
}

// newLimitState builds an empty in-memory limiter state.
func newLimitState() *limitState {
	return &limitState{
		rates:       make(map[string]rateWindow),
		concurrency: make(map[string]int),
	}
}

func (s *limitState) allowRate(now time.Time, clientID, routeKey string, policy RouteLimitPolicy) bool {
	if policy.RequestsPerWindow <= 0 || policy.Window <= 0 {
		return false
	}

	key := limitKey(clientID, routeKey)
	s.mu.Lock()
	defer s.mu.Unlock()

	window := s.rates[key]
	if window.start.IsZero() || !now.Before(window.start.Add(policy.Window)) {
		s.rates[key] = rateWindow{start: now, count: 1}
		return true
	}
	if window.count >= policy.RequestsPerWindow {
		return false
	}
	window.count++
	s.rates[key] = window
	return true
}

func (s *limitState) AllowRate(now time.Time, clientID, routeKey string, policy appgateway.LimitPolicy) bool {
	return s.allowRate(now, clientID, routeKey, RouteLimitPolicy{
		RouteKey:              routeKey,
		RequestsPerWindow:     policy.RequestsPerWindow,
		Window:                policy.Window,
		MaxConcurrentRequests: policy.MaxConcurrentRequests,
		MaxBodyBytes:          policy.MaxBodyBytes,
		Enabled:               true,
	})
}

func (s *limitState) acquireConcurrency(clientID, routeKey string, policy RouteLimitPolicy) (func(), bool) {
	if policy.MaxConcurrentRequests <= 0 {
		return nil, false
	}

	key := limitKey(clientID, routeKey)
	s.mu.Lock()
	if s.concurrency[key] >= policy.MaxConcurrentRequests {
		s.mu.Unlock()
		return nil, false
	}
	s.concurrency[key]++
	s.mu.Unlock()

	var once sync.Once
	release := func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.concurrency[key] <= 1 {
				delete(s.concurrency, key)
				return
			}
			s.concurrency[key]--
		})
	}
	return release, true
}

func (s *limitState) AcquireConcurrency(clientID, routeKey string, policy appgateway.LimitPolicy) (func(), bool) {
	return s.acquireConcurrency(clientID, routeKey, RouteLimitPolicy{
		RouteKey:              routeKey,
		RequestsPerWindow:     policy.RequestsPerWindow,
		Window:                policy.Window,
		MaxConcurrentRequests: policy.MaxConcurrentRequests,
		MaxBodyBytes:          policy.MaxBodyBytes,
		Enabled:               true,
	})
}

func validRouteLimitPolicy(policy RouteLimitPolicy) bool {
	return policy.Enabled &&
		policy.RouteKey != "" &&
		policy.RequestsPerWindow > 0 &&
		policy.Window > 0 &&
		policy.MaxConcurrentRequests > 0 &&
		policy.MaxBodyBytes > 0
}

func validClientRouteLimitOverride(override ClientRouteLimitOverride) bool {
	return override.Enabled &&
		override.ClientID != "" &&
		override.RouteKey != "" &&
		override.RequestsPerWindow > 0 &&
		override.Window > 0 &&
		override.MaxConcurrentRequests > 0 &&
		override.MaxBodyBytes > 0
}

func limitKey(clientID, routeKey string) string {
	return clientID + "\x00" + routeKey
}
