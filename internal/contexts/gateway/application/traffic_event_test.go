package application

import (
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

func TestLifecycleUseCaseClassifyTrafficEventTermination(t *testing.T) {
	tests := []struct {
		name            string
		input           TrafficEventClassificationInput
		wantErrorType   string
		wantBlockReason string
		wantAction      string
	}{
		{
			name: "missing client key",
			input: TrafficEventClassificationInput{
				Direction:         detection.DirectionInput,
				TerminationReason: "missing_client_key",
			},
			wantErrorType:   "missing_client_key",
			wantBlockReason: "missing_client_key",
			wantAction:      "block",
		},
		{
			name: "read request maps to invalid request",
			input: TrafficEventClassificationInput{
				TerminationReason: "read_request_error",
			},
			wantErrorType: "invalid_request",
			wantAction:    "block",
		},
		{
			name: "missing provider maps to missing route error",
			input: TrafficEventClassificationInput{
				TerminationReason: "missing_provider",
			},
			wantErrorType: "missing_route",
			wantAction:    "block",
		},
		{
			name: "stream block reports block reason",
			input: TrafficEventClassificationInput{
				Decision:          policy.Decision{Action: policy.ActionBlock},
				TerminationReason: "stream_blocked",
			},
			wantErrorType:   "stream_blocked",
			wantBlockReason: "stream_blocked",
			wantAction:      "block",
		},
		{
			name: "allow defaults to ok",
			input: TrafficEventClassificationInput{
				Decision: policy.Decision{Action: policy.ActionAllow},
			},
			wantAction: "allow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := LifecycleUseCase{}.ClassifyTrafficEvent(tt.input)
			if classification.ErrorType != tt.wantErrorType {
				t.Fatalf("ErrorType = %q, want %q", classification.ErrorType, tt.wantErrorType)
			}
			if classification.BlockReason != tt.wantBlockReason {
				t.Fatalf("BlockReason = %q, want %q", classification.BlockReason, tt.wantBlockReason)
			}
			if string(classification.Action) != tt.wantAction {
				t.Fatalf("Action = %q, want %q", classification.Action, tt.wantAction)
			}
		})
	}
}

func TestLifecycleUseCaseClassifyTrafficEventBlockedPolicy(t *testing.T) {
	classification := LifecycleUseCase{}.ClassifyTrafficEvent(TrafficEventClassificationInput{
		Direction: detection.DirectionOutput,
		Decision:  policy.Decision{Action: policy.ActionBlock},
	})

	if classification.ErrorType != "blocked_output" {
		t.Fatalf("ErrorType = %q, want blocked_output", classification.ErrorType)
	}
	if classification.BlockReason != "blocked_output" {
		t.Fatalf("BlockReason = %q, want blocked_output", classification.BlockReason)
	}
	if classification.Action != "block" {
		t.Fatalf("Action = %q, want block", classification.Action)
	}
}

func TestLifecycleUseCaseClassifyTrafficEventObservability(t *testing.T) {
	classification := LifecycleUseCase{}.ClassifyTrafficEvent(TrafficEventClassificationInput{
		Direction: detection.DirectionInput,
		Decision: policy.Decision{
			Action: policy.ActionRedact,
			Findings: []detection.Finding{
				{Category: "pii.email"},
			},
			RuleHits: []policy.RuleHit{
				{RuleKey: "rule-a"},
			},
			ModelSignal: policy.ModelSignal{FallbackUsed: true},
		},
		Verdict: verdict.Result{
			FallbackUsed: true,
			MatchedSpans: []verdict.MatchedSpan{
				{Start: 0, End: 4},
			},
		},
	})

	if !classification.FallbackTriggered {
		t.Fatal("FallbackTriggered = false, want true")
	}
	if classification.MatchedSpanCount != 2 {
		t.Fatalf("MatchedSpanCount = %d, want 2", classification.MatchedSpanCount)
	}
	if got := classification.DetectorCategories; len(got) != 1 || got[0] != "pii.email" {
		t.Fatalf("DetectorCategories = %#v, want pii.email", got)
	}
	if got := classification.PolicyRuleIDs; len(got) != 1 || got[0] != "rule-a" {
		t.Fatalf("PolicyRuleIDs = %#v, want rule-a", got)
	}
	if classification.Action != "redact" {
		t.Fatalf("Action = %q, want redact", classification.Action)
	}
}
