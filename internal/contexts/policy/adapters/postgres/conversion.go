package postgres

import (
	"encoding/json"
	"fmt"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	domainpolicy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// detectorConfig encodes detector metadata in the persisted JSON storage shape.
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

// detectorStorageKind normalizes detector kinds to the persisted storage vocabulary.
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

// detectorPolicyKind converts persisted detector metadata back into policy vocabulary.
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

func detectorCategories(raw json.RawMessage) []string {
	var cfg struct {
		Categories []string `json:"categories"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	return append([]string(nil), cfg.Categories...)
}

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

func defaultBundleSource(source string) string {
	if source != "" {
		return source
	}
	return string(domainpolicy.SourceUser)
}

func defaultBundleAction(action string) string {
	if action != "" {
		return action
	}
	return string(domainpolicy.ActionAllow)
}

func defaultJSONObject(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

func remapRequiredID(oldID string, ids map[string]string, label string) (string, error) {
	newID, ok := ids[oldID]
	if !ok {
		return "", fmt.Errorf("missing cloned %s id for %s", label, oldID)
	}
	return newID, nil
}

func remapOptionalID(oldID string, ids map[string]string, label string) (string, error) {
	if oldID == "" {
		return "", nil
	}
	return remapRequiredID(oldID, ids, label)
}
