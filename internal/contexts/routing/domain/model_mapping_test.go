package routing

import "testing"

func TestModelMappingResolveRejectsDifferentRequestedModel(t *testing.T) {
	mapping := ModelMapping{
		Requested: "gpt-public",
		Mapped:    "gpt-policy",
		Upstream:  "gpt-upstream",
	}

	resolved := mapping.Resolve("gpt-other")

	if resolved.Found {
		t.Fatalf("Found = true, want false for a different requested model")
	}
	if resolved.Mapped != "" || resolved.Upstream != "" {
		t.Fatalf("resolved mapping = %+v, want empty models when not found", resolved)
	}
}

func TestModelMappingResolveDefaultsMappedAndUpstreamModels(t *testing.T) {
	tests := []struct {
		name          string
		mapping       ModelMapping
		requested     string
		wantMapped    string
		wantUpstream  string
		wantRequested string
	}{
		{
			name:          "empty mapping keeps requested model",
			requested:     "gpt-public",
			wantMapped:    "gpt-public",
			wantUpstream:  "gpt-public",
			wantRequested: "gpt-public",
		},
		{
			name: "mapped model becomes upstream fallback",
			mapping: ModelMapping{
				Requested: "gpt-public",
				Mapped:    "gpt-policy",
			},
			requested:     "gpt-public",
			wantMapped:    "gpt-policy",
			wantUpstream:  "gpt-policy",
			wantRequested: "gpt-public",
		},
		{
			name: "explicit upstream overrides mapped model",
			mapping: ModelMapping{
				Requested: "gpt-public",
				Mapped:    "gpt-policy",
				Upstream:  "gpt-upstream",
			},
			requested:     "gpt-public",
			wantMapped:    "gpt-policy",
			wantUpstream:  "gpt-upstream",
			wantRequested: "gpt-public",
		},
		{
			name: "wildcard mapping applies upstream model",
			mapping: ModelMapping{
				Upstream: "gpt-upstream",
			},
			requested:     "gpt-public",
			wantMapped:    "gpt-public",
			wantUpstream:  "gpt-upstream",
			wantRequested: "gpt-public",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := tt.mapping.Resolve(tt.requested)

			if !resolved.Found {
				t.Fatal("Found = false, want true")
			}
			if resolved.Requested != tt.wantRequested || resolved.Mapped != tt.wantMapped || resolved.Upstream != tt.wantUpstream {
				t.Fatalf("resolved = %+v, want requested=%q mapped=%q upstream=%q", resolved, tt.wantRequested, tt.wantMapped, tt.wantUpstream)
			}
		})
	}
}
