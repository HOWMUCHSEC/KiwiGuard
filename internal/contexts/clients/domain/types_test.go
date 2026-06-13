package clients

import (
	"testing"
	"time"
)

func TestClientStatusValues(t *testing.T) {
	tests := map[Status]string{
		StatusEnabled:  "enabled",
		StatusDisabled: "disabled",
		StatusRevoked:  "revoked",
	}

	for status, want := range tests {
		if string(status) != want {
			t.Fatalf("status %q = %q, want %q", status, string(status), want)
		}
	}
}

func TestRouteLimitPolicyValueSemantics(t *testing.T) {
	policy := RouteLimitPolicy{
		RouteKey:              "chat",
		RequestsPerWindow:     100,
		Window:                time.Minute,
		MaxConcurrentRequests: 8,
		MaxBodyBytes:          1 << 20,
		Enabled:               true,
	}

	copied := policy
	copied.RequestsPerWindow = 50

	if policy.RequestsPerWindow != 100 {
		t.Fatalf("policy mutated through copy: %d", policy.RequestsPerWindow)
	}
	if copied.RouteKey != "chat" || copied.Window != time.Minute || !copied.Enabled {
		t.Fatalf("copied policy lost fields: %#v", copied)
	}
}

func TestClientRouteLimitOverrideValueSemantics(t *testing.T) {
	override := RouteLimitOverride{
		ClientID:              "client-a",
		RouteKey:              "chat",
		RequestsPerWindow:     10,
		Window:                time.Second,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          4096,
		Enabled:               true,
	}

	if override.ClientID != "client-a" {
		t.Fatalf("ClientID = %q, want client-a", override.ClientID)
	}
	if !override.Enabled {
		t.Fatal("Enabled = false, want true")
	}

	disabled := override
	disabled.Enabled = false
	if disabled.Enabled {
		t.Fatal("disabled override Enabled = true, want false")
	}
	if override.Enabled != true {
		t.Fatal("original override Enabled mutated through copy")
	}
}
