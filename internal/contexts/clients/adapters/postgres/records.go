// Package postgres contains PostgreSQL persistence for gateway clients.
package postgres

import "time"

// GatewayClient contains API client credential metadata for gateway access.
type GatewayClient struct {
	ID         string
	ExternalID string
	Name       string
	Status     string
	KeyPrefix  string
	KeyHash    string
	Notes      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	RevokedAt  *time.Time
}
