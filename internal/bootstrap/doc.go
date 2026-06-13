// Package bootstrap assembles KiwiGuard production dependencies.
//
// Bootstrap is the composition root for executable modes. It may wire concrete
// PostgreSQL, ClickHouse, HTTP, observability, runtime, and event-pipeline
// adapters together, but business rules belong in context domain or
// application packages.
package bootstrap
