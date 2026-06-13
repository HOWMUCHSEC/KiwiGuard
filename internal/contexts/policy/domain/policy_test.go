package policy

import (
	"fmt"
	"testing"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
)

func TestSnapshotEvaluatesScopedDetectorRule(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "built-in",
		Version:       "1.0.0",
		Source:        SourceBuiltIn,
		DefaultAction: ActionAllow,
		Detectors: []detection.Definition{
			{Key: "email", Kind: detection.KindEmail},
		},
		Rules: []Rule{{
			Key:          "block-email-input",
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"email"},
			Scope: Scope{
				RouteKey:  "openai",
				Model:     "gpt-test",
				Direction: detection.DirectionInput,
			},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		RouteKey:  "openai",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
		Text:      "email alice@example.com",
	})
	if decision.Action != ActionBlock {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
	}
	if len(decision.RuleHits) != 1 {
		t.Fatalf("len(RuleHits) = %d, want 1", len(decision.RuleHits))
	}
	if decision.RuleHits[0].RuleKey != "block-email-input" {
		t.Fatalf("RuleKey = %q, want block-email-input", decision.RuleHits[0].RuleKey)
	}
	if decision.SnapshotHash == "" {
		t.Fatal("SnapshotHash is empty")
	}
}

func TestNilSnapshotAllowsAndHasEmptyHash(t *testing.T) {
	var snapshot *Snapshot

	if got := snapshot.Hash(); got != "" {
		t.Fatalf("Hash() = %q, want empty hash", got)
	}
	decision := snapshot.Evaluate(EvaluationRequest{Text: "alice@example.com"})
	if decision.Action != ActionAllow || decision.DefaultAction != ActionAllow {
		t.Fatalf("nil snapshot decision = %+v, want allow/default allow", decision)
	}
}

func TestSnapshotSkipsOutOfScopeRules(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "user",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []Rule{{
			Key:          "shadow-email-output",
			Enabled:      true,
			Severity:     SeverityMedium,
			Action:       ActionShadowLog,
			DetectorKeys: []string{"email"},
			Scope:        Scope{RouteKey: "other", Direction: detection.DirectionOutput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		RouteKey:  "openai",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
		Text:      "alice@example.com",
	})
	if decision.Action != ActionAllow {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionAllow)
	}
	if len(decision.RuleHits) != 0 {
		t.Fatalf("len(RuleHits) = %d, want 0", len(decision.RuleHits))
	}
}

func TestModelSignalCanEscalateAction(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "empty",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		RouteKey:  "openai",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
		Text:      "plain",
		ModelSignal: ModelSignal{
			SuggestedAction: ActionBlock,
			RiskLevel:       "high",
			Categories:      []string{"model.security"},
			Confidence:      0.91,
		},
	})
	if decision.Action != ActionBlock {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
	}
	if !decision.ModelSignalApplied {
		t.Fatal("ModelSignalApplied = false, want true")
	}
}

func TestSnapshotEvaluatesWildcardAndSpecificScopeCandidates(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "scope",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []Rule{
			{
				Key:          "global-shadow",
				Enabled:      true,
				Severity:     SeverityLow,
				Action:       ActionShadowLog,
				DetectorKeys: []string{"email"},
			},
			{
				Key:          "provider-redact",
				Enabled:      true,
				Severity:     SeverityHigh,
				Action:       ActionRedact,
				DetectorKeys: []string{"email"},
				Scope:        Scope{Provider: "openai"},
			},
			{
				Key:          "wrong-model-block",
				Enabled:      true,
				Severity:     SeverityCritical,
				Action:       ActionBlock,
				DetectorKeys: []string{"email"},
				Scope:        Scope{Provider: "openai", Model: "gpt-other"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		RouteKey:  "chat",
		Provider:  "openai",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
		Text:      "alice@example.com",
	})

	if decision.Action != ActionRedact {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionRedact)
	}
	if got, want := ruleHitKeys(decision.RuleHits), []string{"global-shadow", "provider-redact"}; !sameStrings(got, want) {
		t.Fatalf("rule hit keys = %+v, want %+v", got, want)
	}
}

func TestSnapshotDefaultActionUsesHighestPriorityBundleDefault(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{
		{
			Key:           "shadow",
			Version:       "1.0.0",
			Source:        SourceBuiltIn,
			DefaultAction: ActionShadowLog,
		},
		{
			Key:           "redact",
			Version:       "1.0.0",
			Source:        SourceUser,
			DefaultAction: ActionRedact,
		},
		{
			Key:           "allow",
			Version:       "1.0.0",
			Source:        SourceImported,
			DefaultAction: ActionAllow,
		},
	})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{Text: "plain text"})
	if decision.DefaultAction != ActionRedact {
		t.Fatalf("DefaultAction = %q, want %q", decision.DefaultAction, ActionRedact)
	}
	if decision.Action != ActionRedact {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionRedact)
	}
}

func TestSnapshotRuleActionsCannotDowngradeHigherPriorityDefault(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "default-block",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionBlock,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []Rule{{
			Key:          "allow-email",
			Enabled:      true,
			Severity:     SeverityLow,
			Action:       ActionAllow,
			DetectorKeys: []string{"email"},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{Text: "alice@example.com"})
	if decision.Action != ActionBlock {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
	}
	if len(decision.RuleHits) != 1 {
		t.Fatalf("len(RuleHits) = %d, want 1", len(decision.RuleHits))
	}
	if decision.RuleHits[0].Action != ActionAllow {
		t.Fatalf("RuleHits[0].Action = %q, want %q", decision.RuleHits[0].Action, ActionAllow)
	}
}

func TestModelSignalFallbackActionCanEscalateDecision(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "fallback",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionShadowLog,
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		Text: "plain",
		ModelSignal: ModelSignal{
			SuggestedAction: ActionAllow,
			FallbackAction:  ActionBlock,
			FallbackUsed:    true,
		},
	})
	if decision.Action != ActionBlock {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
	}
	if !decision.ModelSignalApplied {
		t.Fatal("ModelSignalApplied = false, want true")
	}
}

func TestCompileSnapshotRejectsMissingDetectorReference(t *testing.T) {
	_, err := CompileSnapshot([]Bundle{{
		Key:           "bad",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
		Rules: []Rule{{
			Key:          "missing",
			Enabled:      true,
			Action:       ActionBlock,
			DetectorKeys: []string{"missing-detector"},
		}},
	}})
	if err == nil {
		t.Fatal("CompileSnapshot() error = nil, want error")
	}
}

func TestCompileSnapshotRejectsInvalidBundleAndRuleFields(t *testing.T) {
	tests := []struct {
		name    string
		bundles []Bundle
	}{
		{
			name:    "missing bundle key",
			bundles: []Bundle{{DefaultAction: ActionAllow}},
		},
		{
			name: "invalid bundle default action",
			bundles: []Bundle{{
				Key:           "bad-default",
				DefaultAction: Action("quarantine"),
			}},
		},
		{
			name: "invalid detector definition",
			bundles: []Bundle{{
				Key:           "bad-detector",
				DefaultAction: ActionAllow,
				Detectors:     []detection.Definition{{Key: "bad", Kind: detection.KindRegex, Pattern: "("}},
			}},
		},
		{
			name: "missing rule key",
			bundles: []Bundle{{
				Key:           "bad-rule",
				DefaultAction: ActionAllow,
				Rules:         []Rule{{Enabled: true, Action: ActionBlock}},
			}},
		},
		{
			name: "invalid rule action",
			bundles: []Bundle{{
				Key:           "bad-rule-action",
				DefaultAction: ActionAllow,
				Rules:         []Rule{{Key: "quarantine", Enabled: true, Action: Action("quarantine")}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := CompileSnapshot(tt.bundles); err == nil {
				t.Fatal("CompileSnapshot() error = nil, want validation error")
			}
		})
	}
}

func TestSnapshotRulesWithoutDetectorsDoNotMatch(t *testing.T) {
	snapshot, err := CompileSnapshot([]Bundle{{
		Key:           "empty-rule",
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
		Rules: []Rule{{
			Key:      "block-without-detectors",
			Enabled:  true,
			Severity: SeverityHigh,
			Action:   ActionBlock,
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	decision := snapshot.Evaluate(EvaluationRequest{Text: "alice@example.com"})
	if decision.Action != ActionAllow {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionAllow)
	}
	if len(decision.RuleHits) != 0 {
		t.Fatalf("len(RuleHits) = %d, want 0", len(decision.RuleHits))
	}
}

func TestEvaluateCachesDetectorFindingsForSharedRules(t *testing.T) {
	detector := &countingDetector{key: "email"}
	snapshot := &Snapshot{
		hash:          "hash",
		defaultAction: ActionAllow,
		bundleKeys:    []string{"test"},
		rulesByScope: map[scopeIndex][]compiledRule{
			{routeKey: "openai", direction: detection.DirectionInput}: {
				{
					bundleKey: "test",
					key:       "block-email",
					severity:  SeverityHigh,
					action:    ActionBlock,
					scope:     Scope{RouteKey: "openai", Direction: detection.DirectionInput},
					detectors: []compiledDetector{{cacheKey: "test/email", detector: detector}},
				},
				{
					bundleKey: "test",
					key:       "shadow-email",
					severity:  SeverityMedium,
					action:    ActionShadowLog,
					scope:     Scope{RouteKey: "openai", Direction: detection.DirectionInput},
					detectors: []compiledDetector{{cacheKey: "test/email", detector: detector}},
				},
			},
		},
	}

	decision := snapshot.Evaluate(EvaluationRequest{
		RouteKey:  "openai",
		Direction: detection.DirectionInput,
		Text:      "alice@example.com",
	})

	if detector.calls != 1 {
		t.Fatalf("detector calls = %d, want 1", detector.calls)
	}
	if len(decision.RuleHits) != 2 {
		t.Fatalf("len(RuleHits) = %d, want 2", len(decision.RuleHits))
	}
	if decision.Action != ActionBlock {
		t.Fatalf("Action = %q, want %q", decision.Action, ActionBlock)
	}
}

func TestEvaluateSingleDetectorRulesAvoidPerRuleFindingAllocations(t *testing.T) {
	const ruleCount = 100

	detector := &countingDetector{key: "email"}
	rules := make([]compiledRule, 0, ruleCount)
	for i := range ruleCount {
		rules = append(rules, compiledRule{
			bundleKey: "test",
			key:       fmt.Sprintf("block-email-%03d", i),
			severity:  SeverityHigh,
			action:    ActionBlock,
			scope:     Scope{RouteKey: "openai", Direction: detection.DirectionInput},
			detectors: []compiledDetector{{cacheKey: "test/email", detector: detector}},
		})
	}
	snapshot := &Snapshot{
		hash:          "hash",
		defaultAction: ActionAllow,
		bundleKeys:    []string{"test"},
		rulesByScope: map[scopeIndex][]compiledRule{
			{routeKey: "openai", direction: detection.DirectionInput}: rules,
		},
	}
	request := EvaluationRequest{
		RouteKey:  "openai",
		Direction: detection.DirectionInput,
		Text:      "alice@example.com",
	}

	allocs := testing.AllocsPerRun(20, func() {
		decision := snapshot.Evaluate(request)
		if len(decision.RuleHits) != ruleCount {
			t.Fatalf("len(RuleHits) = %d, want %d", len(decision.RuleHits), ruleCount)
		}
	})
	if allocs >= 50 {
		t.Fatalf("allocations per evaluation = %.0f, want fewer than 50", allocs)
	}
}

func TestSnapshotHashCanonicalizesEquivalentBundleOrder(t *testing.T) {
	first, err := CompileSnapshot([]Bundle{
		canonicalHashBundleWithKey("secondary", nil, nil),
		canonicalHashBundle([]detection.Definition{
			{Key: "secret", Kind: detection.KindSecret},
			{Key: "email", Kind: detection.KindEmail},
		}, []Rule{
			{
				Key:          "shadow-secret",
				Enabled:      true,
				Severity:     SeverityMedium,
				Action:       ActionShadowLog,
				DetectorKeys: []string{"secret"},
			},
			{
				Key:          "block-email",
				Enabled:      true,
				Severity:     SeverityHigh,
				Action:       ActionBlock,
				DetectorKeys: []string{"email"},
			},
		}),
	})
	if err != nil {
		t.Fatalf("CompileSnapshot(first) error = %v", err)
	}

	second, err := CompileSnapshot([]Bundle{
		canonicalHashBundle([]detection.Definition{
			{Key: "email", Kind: detection.KindEmail},
			{Key: "secret", Kind: detection.KindSecret},
		}, []Rule{
			{
				Key:          "block-email",
				Enabled:      true,
				Severity:     SeverityHigh,
				Action:       ActionBlock,
				DetectorKeys: []string{"email"},
			},
			{
				Key:          "shadow-secret",
				Enabled:      true,
				Severity:     SeverityMedium,
				Action:       ActionShadowLog,
				DetectorKeys: []string{"secret"},
			},
		}),
		canonicalHashBundleWithKey("secondary", []detection.Definition{}, []Rule{}),
	})
	if err != nil {
		t.Fatalf("CompileSnapshot(second) error = %v", err)
	}

	if first.Hash() != second.Hash() {
		t.Fatalf("hash mismatch: %q != %q", first.Hash(), second.Hash())
	}
}

func TestSnapshotHashIgnoresInactiveRules(t *testing.T) {
	activeOnly, err := CompileSnapshot([]Bundle{canonicalHashBundle(
		[]detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		[]Rule{{
			Key:          "block-email",
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"email"},
		}},
	)})
	if err != nil {
		t.Fatalf("CompileSnapshot(activeOnly) error = %v", err)
	}

	withInactiveRule, err := CompileSnapshot([]Bundle{canonicalHashBundle(
		[]detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		[]Rule{
			{
				Key:          "disabled-shadow",
				Enabled:      false,
				Severity:     SeverityLow,
				Action:       ActionShadowLog,
				DetectorKeys: []string{"email"},
			},
			{
				Key:          "block-email",
				Enabled:      true,
				Severity:     SeverityHigh,
				Action:       ActionBlock,
				DetectorKeys: []string{"email"},
			},
		},
	)})
	if err != nil {
		t.Fatalf("CompileSnapshot(withInactiveRule) error = %v", err)
	}

	if activeOnly.Hash() != withInactiveRule.Hash() {
		t.Fatalf("hash mismatch with inactive rule: %q != %q", activeOnly.Hash(), withInactiveRule.Hash())
	}
}

func TestSnapshotHashCanonicalizesCategoryAndDetectorKeyOrder(t *testing.T) {
	first, err := CompileSnapshot([]Bundle{canonicalHashBundle(
		[]detection.Definition{
			{Key: "secret", Kind: detection.KindSecret, Categories: []string{"secret.token", "credential"}},
			{Key: "email", Kind: detection.KindEmail, Categories: []string{"pii.email", "contact"}},
		},
		[]Rule{{
			Key:          "block-sensitive",
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"secret", "email"},
		}},
	)})
	if err != nil {
		t.Fatalf("CompileSnapshot(first) error = %v", err)
	}

	second, err := CompileSnapshot([]Bundle{canonicalHashBundle(
		[]detection.Definition{
			{Key: "email", Kind: detection.KindEmail, Categories: []string{"contact", "pii.email"}},
			{Key: "secret", Kind: detection.KindSecret, Categories: []string{"credential", "secret.token"}},
		},
		[]Rule{{
			Key:          "block-sensitive",
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"email", "secret"},
		}},
	)})
	if err != nil {
		t.Fatalf("CompileSnapshot(second) error = %v", err)
	}

	if first.Hash() != second.Hash() {
		t.Fatalf("hash mismatch: %q != %q", first.Hash(), second.Hash())
	}
}

func TestCompileSnapshotDoesNotMutateBundleSlicesDuringCanonicalHashing(t *testing.T) {
	bundle := canonicalHashBundle(
		[]detection.Definition{
			{Key: "secret", Kind: detection.KindSecret, Categories: []string{"z", "a"}},
			{Key: "email", Kind: detection.KindEmail},
		},
		[]Rule{{
			Key:          "block-sensitive",
			Enabled:      true,
			Severity:     SeverityHigh,
			Action:       ActionBlock,
			DetectorKeys: []string{"secret", "email"},
		}},
	)

	_, err := CompileSnapshot([]Bundle{bundle})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}
	if got, want := bundle.Detectors[0].Categories, []string{"z", "a"}; !sameStringsInOrder(got, want) {
		t.Fatalf("detector categories = %+v, want original order %+v", got, want)
	}
	if got, want := bundle.Rules[0].DetectorKeys, []string{"secret", "email"}; !sameStringsInOrder(got, want) {
		t.Fatalf("rule detector keys = %+v, want original order %+v", got, want)
	}
}

func TestMatchesScopeRejectsEachSpecificMismatch(t *testing.T) {
	req := EvaluationRequest{
		RouteKey:  "openai",
		Provider:  "primary",
		Model:     "gpt-test",
		Direction: detection.DirectionInput,
	}
	tests := []struct {
		name  string
		scope Scope
		want  bool
	}{
		{name: "wildcard matches", scope: Scope{}, want: true},
		{name: "exact matches", scope: Scope{RouteKey: "openai", Provider: "primary", Model: "gpt-test", Direction: detection.DirectionInput}, want: true},
		{name: "route mismatch", scope: Scope{RouteKey: "other"}, want: false},
		{name: "provider mismatch", scope: Scope{Provider: "secondary"}, want: false},
		{name: "model mismatch", scope: Scope{Model: "gpt-other"}, want: false},
		{name: "direction mismatch", scope: Scope{Direction: detection.DirectionOutput}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesScope(tt.scope, req); got != tt.want {
				t.Fatalf("matchesScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchRuleCachesSingleAndMultipleDetectors(t *testing.T) {
	first := &countingDetector{key: "first"}
	second := &countingDetector{key: "second"}
	input := detection.Input{Text: "alice@example.com"}
	cache := map[string][]detection.Finding{}

	singleFindings := matchRule(compiledRule{detectors: []compiledDetector{{cacheKey: "first", detector: first}}}, input, cache)
	_ = matchRule(compiledRule{detectors: []compiledDetector{{cacheKey: "first", detector: first}}}, input, cache)
	if len(singleFindings) != 1 || first.calls != 1 {
		t.Fatalf("single detector findings=%+v calls=%d, want one cached match", singleFindings, first.calls)
	}

	multiFindings := matchRule(compiledRule{detectors: []compiledDetector{
		{cacheKey: "first", detector: first},
		{cacheKey: "second", detector: second},
	}}, input, cache)
	if len(multiFindings) != 2 {
		t.Fatalf("multi detector findings = %d, want 2", len(multiFindings))
	}
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("detector calls = %d/%d, want cached first and one second call", first.calls, second.calls)
	}
	if got := matchRule(compiledRule{}, input, cache); got != nil {
		t.Fatalf("matchRule(empty) = %+v, want nil", got)
	}
}

func TestScopeAndPriorityHelpersUseFallbacks(t *testing.T) {
	if got := stringScopeValues(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("stringScopeValues(empty) = %+v, want wildcard only", got)
	}
	if got := stringScopeValues("openai"); len(got) != 2 || got[0] != "openai" || got[1] != "" {
		t.Fatalf("stringScopeValues(openai) = %+v, want specific then wildcard", got)
	}
	if got := directionScopeValues(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("directionScopeValues(empty) = %+v, want wildcard only", got)
	}
	if got := directionScopeValues(detection.DirectionInput); len(got) != 2 || got[0] != detection.DirectionInput || got[1] != "" {
		t.Fatalf("directionScopeValues(input) = %+v, want specific then wildcard", got)
	}
	if actionPriority(Action("unknown")) != actionPriority(ActionAllow) {
		t.Fatal("unknown action priority did not fall back to allow priority")
	}
	if got := higherPriority(ActionShadowLog, ActionBlock); got != ActionBlock {
		t.Fatalf("higherPriority(shadow, block) = %q, want block", got)
	}
	if got := higherPriority(ActionBlock, ActionAllow); got != ActionBlock {
		t.Fatalf("higherPriority(block, allow) = %q, want block", got)
	}
}

type countingDetector struct {
	key   string
	calls int
}

func (d *countingDetector) Key() string {
	return d.key
}

func (d *countingDetector) Match(input detection.Input) []detection.Finding {
	d.calls++
	return []detection.Finding{{
		DetectorKey: d.key,
		Category:    "pii.email",
		Start:       0,
		End:         len(input.Text),
		ValueHash:   "hash",
	}}
}

func canonicalHashBundle(defs []detection.Definition, rules []Rule) Bundle {
	return canonicalHashBundleWithKey("canonical", defs, rules)
}

func canonicalHashBundleWithKey(key string, defs []detection.Definition, rules []Rule) Bundle {
	return Bundle{
		Key:           key,
		Version:       "1.0.0",
		Source:        SourceUser,
		DefaultAction: ActionAllow,
		Detectors:     defs,
		Rules:         rules,
	}
}

func ruleHitKeys(hits []RuleHit) []string {
	keys := make([]string, 0, len(hits))
	for _, hit := range hits {
		keys = append(keys, hit.RuleKey)
	}
	return keys
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
}

func sameStringsInOrder(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
