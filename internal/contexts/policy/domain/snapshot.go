package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
)

// Snapshot holds compiled policy state for repeated evaluation.
type Snapshot struct {
	hash          string
	bundleKeys    []string
	defaultAction Action
	rules         []compiledRule
	rulesByScope  map[scopeIndex][]compiledRule
}

type compiledRule struct {
	bundleKey string
	key       string
	severity  Severity
	action    Action
	scope     Scope
	detectors []compiledDetector
}

type compiledDetector struct {
	cacheKey string
	detector detection.Detector
}

type scopeIndex struct {
	routeKey  string
	provider  string
	model     string
	direction detection.Direction
}

// CompileSnapshot validates bundles and compiles detectors into an immutable snapshot.
func CompileSnapshot(bundles []Bundle) (*Snapshot, error) {
	activeBundles := make([]Bundle, 0, len(bundles))
	compiledRules := make([]compiledRule, 0)
	bundleKeys := make([]string, 0, len(bundles))
	defaultAction := ActionAllow

	for bundleIndex, bundle := range bundles {
		if bundle.Key == "" {
			return nil, fmt.Errorf("compile snapshot: bundle %d key is required", bundleIndex)
		}
		if !validAction(bundle.DefaultAction) {
			return nil, fmt.Errorf("compile snapshot: bundle %s default action %q is invalid", bundle.Key, bundle.DefaultAction)
		}

		compiledDetectors, err := compileDetectorMap(bundle)
		if err != nil {
			return nil, err
		}

		activeBundle := bundle
		activeBundle.Rules = activeRules(bundle.Rules)
		for _, rule := range activeBundle.Rules {
			if rule.Key == "" {
				return nil, fmt.Errorf("compile snapshot: bundle %s rule key is required", bundle.Key)
			}
			if !validAction(rule.Action) {
				return nil, fmt.Errorf("compile snapshot: bundle %s rule %s action %q is invalid", bundle.Key, rule.Key, rule.Action)
			}

			ruleDetectors, err := detectorsForRule(bundle.Key, rule, compiledDetectors)
			if err != nil {
				return nil, err
			}
			compiledRules = append(compiledRules, compiledRule{
				bundleKey: bundle.Key,
				key:       rule.Key,
				severity:  rule.Severity,
				action:    rule.Action,
				scope:     rule.Scope,
				detectors: ruleDetectors,
			})
		}

		defaultAction = higherPriority(defaultAction, bundle.DefaultAction)
		bundleKeys = append(bundleKeys, bundle.Key)
		activeBundles = append(activeBundles, activeBundle)
	}

	hash, err := hashBundles(activeBundles)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		hash:          hash,
		bundleKeys:    append([]string(nil), bundleKeys...),
		defaultAction: defaultAction,
		rules:         compiledRules,
		rulesByScope:  indexRules(compiledRules),
	}, nil
}

// Evaluate evaluates request text and model signals against the snapshot.
func (s *Snapshot) Evaluate(req EvaluationRequest) Decision {
	if s == nil {
		return Decision{Action: ActionAllow, DefaultAction: ActionAllow}
	}

	action := s.defaultAction
	decision := Decision{
		Action:        action,
		DefaultAction: s.defaultAction,
		ModelSignal:   req.ModelSignal,
		SnapshotHash:  s.hash,
		BundleKeys:    append([]string(nil), s.bundleKeys...),
	}
	input := detection.Input{Direction: req.Direction, Text: req.Text}
	findingCache := make(map[string][]detection.Finding)

	s.forEachCandidateRule(req, func(rule compiledRule) {
		if !matchesScope(rule.scope, req) {
			return
		}

		findings := matchRule(rule, input, findingCache)
		if len(findings) == 0 {
			return
		}

		action = higherPriority(action, rule.action)
		decision.RuleHits = append(decision.RuleHits, RuleHit{
			BundleKey: rule.bundleKey,
			RuleKey:   rule.key,
			Severity:  rule.severity,
			Action:    rule.action,
			Findings:  findings,
		})
		decision.Findings = append(decision.Findings, findings...)
	})

	modelAction := req.ModelSignal.SuggestedAction
	if req.ModelSignal.FallbackUsed {
		modelAction = req.ModelSignal.FallbackAction
	}
	if validAction(modelAction) && actionPriority(modelAction) > actionPriority(action) {
		action = modelAction
		decision.ModelSignalApplied = true
	}

	decision.Action = action
	return decision
}

// Hash returns the SHA-256 hash of the canonical active bundle JSON.
func (s *Snapshot) Hash() string {
	if s == nil {
		return ""
	}
	return s.hash
}

func compileDetectorMap(bundle Bundle) (map[string]compiledDetector, error) {
	compiled := make(map[string]compiledDetector, len(bundle.Detectors))
	for _, def := range bundle.Detectors {
		detector, err := detection.Compile(def)
		if err != nil {
			return nil, fmt.Errorf("compile snapshot: bundle %s detector %s: %w", bundle.Key, def.Key, err)
		}
		compiled[def.Key] = compiledDetector{
			cacheKey: bundle.Key + "/" + def.Key,
			detector: detector,
		}
	}
	return compiled, nil
}

func detectorsForRule(bundleKey string, rule Rule, compiled map[string]compiledDetector) ([]compiledDetector, error) {
	ruleDetectors := make([]compiledDetector, 0, len(rule.DetectorKeys))
	for _, detectorKey := range rule.DetectorKeys {
		detector, ok := compiled[detectorKey]
		if !ok {
			return nil, fmt.Errorf("compile snapshot: bundle %s rule %s references missing detector %s", bundleKey, rule.Key, detectorKey)
		}
		ruleDetectors = append(ruleDetectors, detector)
	}
	return ruleDetectors, nil
}

func activeRules(rules []Rule) []Rule {
	active := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		active = append(active, rule)
	}
	return active
}

func hashBundles(bundles []Bundle) (string, error) {
	encoded, err := json.Marshal(canonicalBundles(bundles))
	if err != nil {
		return "", fmt.Errorf("compile snapshot: marshal active bundles: %w", err)
	}

	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalBundles(bundles []Bundle) []Bundle {
	canonical := make([]Bundle, len(bundles))
	for i, bundle := range bundles {
		canonical[i] = canonicalBundle(bundle)
	}
	sort.Slice(canonical, func(i int, j int) bool {
		if canonical[i].Key != canonical[j].Key {
			return canonical[i].Key < canonical[j].Key
		}
		return canonical[i].Version < canonical[j].Version
	})
	return canonical
}

func canonicalBundle(bundle Bundle) Bundle {
	bundle.Detectors = canonicalDetectors(bundle.Detectors)
	bundle.Rules = canonicalRules(bundle.Rules)
	return bundle
}

func canonicalDetectors(defs []detection.Definition) []detection.Definition {
	canonical := make([]detection.Definition, len(defs))
	for i, def := range defs {
		canonical[i] = def
		canonical[i].Categories = sortedStrings(def.Categories)
	}
	sort.Slice(canonical, func(i int, j int) bool {
		if canonical[i].Key != canonical[j].Key {
			return canonical[i].Key < canonical[j].Key
		}
		if canonical[i].Kind != canonical[j].Kind {
			return canonical[i].Kind < canonical[j].Kind
		}
		return canonical[i].Pattern < canonical[j].Pattern
	})
	return canonical
}

func canonicalRules(rules []Rule) []Rule {
	canonical := make([]Rule, len(rules))
	for i, rule := range rules {
		canonical[i] = rule
		canonical[i].DetectorKeys = sortedStrings(rule.DetectorKeys)
	}
	sort.Slice(canonical, func(i int, j int) bool {
		if canonical[i].Key != canonical[j].Key {
			return canonical[i].Key < canonical[j].Key
		}
		if canonical[i].Action != canonical[j].Action {
			return canonical[i].Action < canonical[j].Action
		}
		return canonical[i].Severity < canonical[j].Severity
	})
	return canonical
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted
}

func indexRules(rules []compiledRule) map[scopeIndex][]compiledRule {
	index := make(map[scopeIndex][]compiledRule, len(rules))
	for _, rule := range rules {
		key := scopeIndex{
			routeKey:  rule.scope.RouteKey,
			provider:  rule.scope.Provider,
			model:     rule.scope.Model,
			direction: rule.scope.Direction,
		}
		index[key] = append(index[key], rule)
	}
	return index
}

func (s *Snapshot) forEachCandidateRule(req EvaluationRequest, visit func(compiledRule)) {
	for _, routeKey := range stringScopeValues(req.RouteKey) {
		for _, provider := range stringScopeValues(req.Provider) {
			for _, model := range stringScopeValues(req.Model) {
				for _, direction := range directionScopeValues(req.Direction) {
					key := scopeIndex{
						routeKey:  routeKey,
						provider:  provider,
						model:     model,
						direction: direction,
					}
					for _, rule := range s.rulesByScope[key] {
						visit(rule)
					}
				}
			}
		}
	}
}

func stringScopeValues(value string) []string {
	if value == "" {
		return []string{""}
	}
	return []string{value, ""}
}

func directionScopeValues(value detection.Direction) []detection.Direction {
	if value == "" {
		return []detection.Direction{""}
	}
	return []detection.Direction{value, ""}
}

func matchesScope(scope Scope, req EvaluationRequest) bool {
	if scope.RouteKey != "" && scope.RouteKey != req.RouteKey {
		return false
	}
	if scope.Provider != "" && scope.Provider != req.Provider {
		return false
	}
	if scope.Model != "" && scope.Model != req.Model {
		return false
	}
	if scope.Direction != "" && scope.Direction != req.Direction {
		return false
	}
	return true
}

func matchRule(rule compiledRule, input detection.Input, cache map[string][]detection.Finding) []detection.Finding {
	if len(rule.detectors) == 0 {
		return nil
	}
	if len(rule.detectors) == 1 {
		detector := rule.detectors[0]
		cached, ok := cache[detector.cacheKey]
		if !ok {
			cached = detector.detector.Match(input)
			cache[detector.cacheKey] = cached
		}
		return cached
	}

	var findings []detection.Finding
	for _, detector := range rule.detectors {
		cached, ok := cache[detector.cacheKey]
		if !ok {
			cached = detector.detector.Match(input)
			cache[detector.cacheKey] = cached
		}
		findings = append(findings, cached...)
	}
	return findings
}

func validAction(action Action) bool {
	_, ok := actionPriorities[action]
	return ok
}

func higherPriority(left Action, right Action) Action {
	if actionPriority(right) > actionPriority(left) {
		return right
	}
	return left
}

func actionPriority(action Action) int {
	priority, ok := actionPriorities[action]
	if !ok {
		return actionPriorities[ActionAllow]
	}
	return priority
}

var actionPriorities = map[Action]int{
	ActionAllow:     0,
	ActionShadowLog: 1,
	ActionRedact:    2,
	ActionBlock:     3,
}
