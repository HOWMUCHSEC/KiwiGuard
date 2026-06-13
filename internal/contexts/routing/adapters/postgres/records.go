// Package postgres contains PostgreSQL persistence for gateway routing configuration.
package postgres

import (
	"encoding/json"
	"time"
)

// Route maps gateway traffic to a provider and model mapping.
type Route struct {
	ID             string
	Name           string
	Enabled        bool
	Priority       int
	Method         string
	Path           string
	PathPrefix     string
	Provider       string
	UpstreamModel  string
	ModelMappingID string
	ExecutionMode  string
	FallbackAction string
}

// Provider contains upstream model provider configuration.
type Provider struct {
	ID                   string
	Name                 string
	BaseURL              string
	CredentialRef        string
	Timeout              time.Duration
	ProviderType         string
	Headers              json.RawMessage
	RetryConfig          json.RawMessage
	CircuitBreakerConfig json.RawMessage
	Capabilities         json.RawMessage
}

// ModelMapping maps requested model names to provider model names.
type ModelMapping struct {
	ID               string
	Name             string
	SourceModel      string
	TargetProviderID string
	TargetModel      string
	Parameters       json.RawMessage
}

// VerdictProvider contains security verdict provider configuration.
type VerdictProvider struct {
	ID                   string
	Name                 string
	Adapter              string
	Endpoint             string
	CredentialRef        string
	ModelName            string
	Timeout              time.Duration
	AdapterConfig        json.RawMessage
	RetryConfig          json.RawMessage
	CircuitBreakerConfig json.RawMessage
	MaxConcurrency       int
	Enabled              bool
}

// RouteVerdictProviderBinding connects a route to a verdict provider.
type RouteVerdictProviderBinding struct {
	ID                string
	RouteID           string
	VerdictProviderID string
	Enabled           bool
	ExecutionMode     string
	Priority          int
}
