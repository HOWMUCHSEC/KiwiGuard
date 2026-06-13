package configstore

import (
	"encoding/json"
	"time"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// toPolicyBundles rebuilds enabled policy bundles from persisted PostgreSQL records.
func toPolicyBundles(bundles []policystore.Bundle) []policy.Bundle {
	out := make([]policy.Bundle, 0, len(bundles))
	for _, bundle := range bundles {
		if !bundle.Enabled {
			continue
		}
		defs := make([]detection.Definition, 0, len(bundle.Detectors))
		for _, detector := range bundle.Detectors {
			if !detector.Enabled {
				continue
			}
			defs = append(defs, detection.Definition{
				Key:        detector.Key,
				Kind:       detection.Kind(detector.Kind),
				Pattern:    detector.Pattern,
				Categories: append([]string(nil), detector.Categories...),
			})
		}
		rules := make([]policy.Rule, 0, len(bundle.Rules))
		for _, rule := range bundle.Rules {
			scope := policy.Scope{}
			if len(rule.Scopes) > 0 {
				scope.Model = rule.Scopes[0].Model
				scope.Direction = detection.Direction(rule.Scopes[0].Direction)
			}
			rules = append(rules, policy.Rule{
				Key:          rule.Key,
				Enabled:      rule.Enabled,
				Severity:     policy.Severity(rule.Severity),
				Action:       policy.Action(rule.Action),
				DetectorKeys: append([]string(nil), rule.DetectorKeys...),
				Scope:        scope,
			})
		}
		defaultAction := policy.Action(bundle.DefaultAction)
		if defaultAction == "" {
			defaultAction = policy.ActionAllow
		}
		out = append(out, policy.Bundle{
			Key:           bundle.Key,
			Version:       bundle.Version,
			Source:        policy.Source(bundle.Source),
			DefaultAction: defaultAction,
			Detectors:     defs,
			Rules:         rules,
		})
	}
	return out
}

// detectorConfig preserves the historical JSON payload used for detector metadata in persisted rows.
func detectorConfig(kind string, categories []string) json.RawMessage {
	encoded, err := json.Marshal(struct {
		Kind       string   `json:"kind,omitempty"`
		Categories []string `json:"categories"`
	}{Kind: kind, Categories: categories})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

// detectorStorageKind normalizes policy detector kinds to their persisted representation.
func detectorStorageKind(kind string) string {
	switch detection.Kind(kind) {
	case detection.KindRegex:
		return string(detection.KindRegex)
	case detection.KindEmail, detection.KindPhone, detection.KindPaymentCard, detection.KindSecret:
		return "builtin"
	default:
		return kind
	}
}

// detectorPolicyKind restores a policy detector kind from persisted detector metadata.
func detectorPolicyKind(storageKind string, raw json.RawMessage) string {
	if storageKind != "builtin" {
		return storageKind
	}
	var cfg struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.Kind == "" {
		return storageKind
	}
	return cfg.Kind
}

// detectorCategories extracts persisted detector categories from raw JSON metadata.
func detectorCategories(raw json.RawMessage) []string {
	var cfg struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	return append([]string(nil), cfg.Categories...)
}

// storageDirection normalizes policy directions to the persisted routing vocabulary.
func storageDirection(direction string) string {
	switch direction {
	case string(detection.DirectionInput), "request":
		return "request"
	case string(detection.DirectionOutput), "response":
		return "response"
	case "both":
		return "both"
	default:
		return "both"
	}
}

// policyDirection translates persisted routing directions back into policy-domain vocabulary.
func policyDirection(direction string) string {
	switch direction {
	case "request":
		return string(detection.DirectionInput)
	case "response":
		return string(detection.DirectionOutput)
	default:
		return ""
	}
}

// defaultJSONObject guarantees an object-shaped JSON payload for optional persisted metadata.
func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

// durationMillis truncates a duration to whole milliseconds for persisted records.
func durationMillis(d time.Duration) int {
	return int(d / time.Millisecond)
}
