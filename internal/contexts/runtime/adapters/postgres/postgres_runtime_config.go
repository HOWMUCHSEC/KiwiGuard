package runtime

import (
	configrevision "github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

// postgresRuntimeConfig is the runtime context's PostgreSQL aggregate.
//
// It intentionally lives in the runtime context adapter instead of the shared
// configstore root package so runtime persistence does not depend on a
// cross-context facade data model.
type postgresRuntimeConfig struct {
	Revision                     configrevision.ConfigRevision
	Routes                       []routingstore.Route
	Providers                    []routingstore.Provider
	ModelMappings                []routingstore.ModelMapping
	VerdictProviders             []routingstore.VerdictProvider
	RouteVerdictProviderBindings []routingstore.RouteVerdictProviderBinding
	PolicyBundles                []policystore.Bundle
	Sinks                        []observabilitystore.Sink
	Retention                    []observabilitystore.RetentionPolicy
	RawCapture                   []observabilitystore.RawCapturePolicy
	GatewayClients               []clientstore.GatewayClient
	RouteLimitPolicies           []limitstore.RoutePolicy
	ClientRouteLimitOverrides    []limitstore.ClientRouteOverride
}
