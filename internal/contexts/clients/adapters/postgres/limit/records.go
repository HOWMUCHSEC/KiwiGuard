// Package limit contains PostgreSQL record shapes for gateway limits.
package limit

// RoutePolicy contains default rate and size limits for a route.
type RoutePolicy struct {
	ID                    string
	RouteID               string
	RequestsPerWindow     int
	WindowSeconds         int
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}

// ClientRouteOverride contains client-specific route limits.
type ClientRouteOverride struct {
	ID                    string
	ClientID              string
	RouteID               string
	RequestsPerWindow     int
	WindowSeconds         int
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}
