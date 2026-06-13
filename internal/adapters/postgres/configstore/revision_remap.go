package configstore

import "fmt"

// remapRequiredID resolves an identifier through a clone map and fails when no mapping exists.
func remapRequiredID(oldID string, ids map[string]string, label string) (string, error) {
	newID, ok := ids[oldID]
	if !ok {
		return "", fmt.Errorf("clone active config: %s %s was not cloned", label, oldID)
	}
	return newID, nil
}

// remapOptionalID resolves an identifier through a clone map while preserving empty values.
func remapOptionalID(oldID string, ids map[string]string, label string) (string, error) {
	if oldID == "" {
		return "", nil
	}
	return remapRequiredID(oldID, ids, label)
}
