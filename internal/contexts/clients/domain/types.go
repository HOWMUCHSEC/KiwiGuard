// Package clients defines gateway client access and limit concepts.
package clients

import "time"

// Status identifies whether a gateway client key can authenticate.
type Status string

const (
	// StatusEnabled permits requests authenticated with the client key.
	StatusEnabled Status = "enabled"
	// StatusDisabled rejects requests until the client key is re-enabled.
	StatusDisabled Status = "disabled"
	// StatusRevoked rejects requests for permanently revoked client keys.
	StatusRevoked Status = "revoked"
)

// Client contains gateway client key metadata and stored verification material.
type Client struct {
	ID        string
	Name      string
	Status    Status
	KeyPrefix string
	KeyHash   string
}

// KeyMaterial contains the key prefix and hash that can be stored.
type KeyMaterial struct {
	Prefix string
	Hash   string
}

// RouteLimitPolicy controls gateway request limits for a route.
type RouteLimitPolicy struct {
	RouteKey              string
	RequestsPerWindow     int
	Window                time.Duration
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}

// RouteLimitOverride controls gateway request limits for one client on one route.
type RouteLimitOverride struct {
	ClientID              string
	RouteKey              string
	RequestsPerWindow     int
	Window                time.Duration
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}
