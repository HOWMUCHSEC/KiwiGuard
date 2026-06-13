package policy

import (
	"fmt"
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
)

func BenchmarkEvaluateTenThousandRules(b *testing.B) {
	benchmarkEvaluateRules(b, 10_000)
}

func BenchmarkEvaluateFiftyThousandRules(b *testing.B) {
	benchmarkEvaluateRules(b, 50_000)
}

func benchmarkEvaluateRules(b *testing.B, ruleCount int) {
	b.Helper()

	snapshot, err := CompileSnapshot([]Bundle{benchmarkBundle(ruleCount)})
	if err != nil {
		b.Fatalf("CompileSnapshot() error = %v", err)
	}

	req := EvaluationRequest{
		RouteKey:  "openai",
		Provider:  "openai",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
		Text:      "contact alice@example.com for details",
	}

	b.ReportAllocs()
	for b.Loop() {
		decision := snapshot.Evaluate(req)
		if decision.Action != ActionBlock {
			b.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
		}
	}
}

func benchmarkBundle(ruleCount int) Bundle {
	rules := make([]Rule, 0, ruleCount)
	for i := range ruleCount {
		rules = append(rules, Rule{
			Key:          fmt.Sprintf("rule-%05d", i),
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"email"},
			Scope: Scope{
				RouteKey:  "openai",
				Provider:  "openai",
				Model:     "gpt-test",
				Direction: detection.DirectionInput,
			},
		})
	}

	return Bundle{
		Key:           "benchmark",
		Version:       "1.0.0",
		Source:        SourceBuiltIn,
		DefaultAction: ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules:         rules,
	}
}
