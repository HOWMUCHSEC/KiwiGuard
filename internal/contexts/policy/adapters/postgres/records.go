// Package postgres contains PostgreSQL persistence for policy configuration.
package postgres

import "encoding/json"

// Bundle contains policy detectors and rules loaded from PostgreSQL.
type Bundle struct {
	ID            string
	Key           string
	Version       string
	Source        string
	Description   string
	DefaultAction string
	Enabled       bool
	Metadata      json.RawMessage
	Detectors     []Detector
	Rules         []Rule
}

// Detector configures one policy detector.
type Detector struct {
	ID         string
	Key        string
	Kind       string
	Pattern    string
	Categories []string
	Config     json.RawMessage
	Enabled    bool
}

// Rule connects detector findings to an action.
type Rule struct {
	ID           string
	Key          string
	Description  string
	Enabled      bool
	Severity     string
	Action       string
	Priority     int
	Config       json.RawMessage
	DetectorKeys []string
	Scopes       []RuleScope
}

// RuleScope limits a rule to traffic metadata.
type RuleScope struct {
	RouteID    string
	ProviderID string
	Model      string
	Direction  string
}

// ActivationRequest activates the named policy bundles.
type ActivationRequest struct {
	Keys         []string
	Actor        string
	Reason       string
	SnapshotHash string
}

// ActivationResult describes a completed policy activation.
type ActivationResult struct {
	RevisionNumber int64
	SnapshotHash   string
	ActiveKeys     []string
}
