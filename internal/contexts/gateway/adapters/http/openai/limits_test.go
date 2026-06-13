package openai

import (
	"testing"
	"time"
)

func TestLimitResolverUsesClientOverrideBeforeRouteDefault(t *testing.T) {
	resolver := newLimitResolver([]Route{{Key: "chat", RequireClientAuth: true}}, []RouteLimitPolicy{{
		RouteKey:              "chat",
		RequestsPerWindow:     10,
		Window:                time.Minute,
		MaxConcurrentRequests: 2,
		MaxBodyBytes:          1024,
		Enabled:               true,
	}}, []ClientRouteLimitOverride{{
		ClientID:              "client-a",
		RouteKey:              "chat",
		RequestsPerWindow:     3,
		Window:                time.Minute,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          512,
		Enabled:               true,
	}})

	if !resolver.routeProtected("chat") {
		t.Fatal("routeProtected(chat) = false, want true")
	}
	policy, ok := resolver.effectivePolicy("client-a", "chat")
	if !ok {
		t.Fatal("effectivePolicy() ok = false, want true")
	}
	if policy.RequestsPerWindow != 3 || policy.MaxConcurrentRequests != 1 || policy.MaxBodyBytes != 512 {
		t.Fatalf("effective policy = %+v, want client override", policy)
	}

	defaultPolicy, ok := resolver.effectivePolicy("client-b", "chat")
	if !ok {
		t.Fatal("effectivePolicy(default) ok = false, want true")
	}
	if defaultPolicy.RequestsPerWindow != 10 || defaultPolicy.MaxConcurrentRequests != 2 || defaultPolicy.MaxBodyBytes != 1024 {
		t.Fatalf("default policy = %+v, want route default", defaultPolicy)
	}
}

func TestLimitResolverRejectsProtectedRouteWithoutDefault(t *testing.T) {
	resolver := newLimitResolver([]Route{{Key: "chat", RequireClientAuth: true}}, nil, nil)

	if !resolver.routeProtected("chat") {
		t.Fatal("routeProtected(chat) = false, want true")
	}
	if policy, ok := resolver.effectivePolicy("client-a", "chat"); ok {
		t.Fatalf("effectivePolicy() = %+v, true; want no policy", policy)
	}
}

func TestLimitResolverRejectsOverrideWithoutRouteDefault(t *testing.T) {
	resolver := newLimitResolver([]Route{{Key: "chat", RequireClientAuth: true}}, nil, []ClientRouteLimitOverride{{
		ClientID:              "client-a",
		RouteKey:              "chat",
		RequestsPerWindow:     3,
		Window:                time.Minute,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          512,
		Enabled:               true,
	}})

	if !resolver.routeProtected("chat") {
		t.Fatal("routeProtected(chat) = false, want true")
	}
	if policy, ok := resolver.effectivePolicy("client-a", "chat"); ok {
		t.Fatalf("effectivePolicy() = %+v, true; want no policy without route default", policy)
	}
}

func TestRateLimiterRejectsAfterWindowCapacity(t *testing.T) {
	state := newLimitState()
	now := time.Unix(100, 0)
	policy := RouteLimitPolicy{
		RouteKey:          "chat",
		RequestsPerWindow: 2,
		Window:            time.Minute,
		Enabled:           true,
	}

	if !state.allowRate(now, "client-a", "chat", policy) {
		t.Fatal("first request rejected, want allowed")
	}
	if !state.allowRate(now.Add(time.Second), "client-a", "chat", policy) {
		t.Fatal("second request rejected, want allowed")
	}
	if state.allowRate(now.Add(2*time.Second), "client-a", "chat", policy) {
		t.Fatal("third request allowed, want rejected")
	}
	if !state.allowRate(now.Add(time.Minute), "client-a", "chat", policy) {
		t.Fatal("request in next window rejected, want allowed")
	}
}

func TestConcurrencyLimiterRejectsWhenFullAndReleases(t *testing.T) {
	state := newLimitState()
	policy := RouteLimitPolicy{
		RouteKey:              "chat",
		MaxConcurrentRequests: 1,
		Enabled:               true,
	}

	release, ok := state.acquireConcurrency("client-a", "chat", policy)
	if !ok {
		t.Fatal("first acquire ok = false, want true")
	}
	if secondRelease, ok := state.acquireConcurrency("client-a", "chat", policy); ok {
		secondRelease()
		t.Fatal("second acquire ok = true, want false")
	}

	release()
	release()

	nextRelease, ok := state.acquireConcurrency("client-a", "chat", policy)
	if !ok {
		t.Fatal("acquire after release ok = false, want true")
	}
	nextRelease()
	nextRelease()

	finalRelease, ok := state.acquireConcurrency("client-a", "chat", policy)
	if !ok {
		t.Fatal("acquire after idempotent release ok = false, want true")
	}
	finalRelease()
}
