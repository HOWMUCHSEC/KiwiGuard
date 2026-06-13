package postgres

import (
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

// configSnapshot contains only the active config graph needed by the control adapter.
type configSnapshot struct {
	Routes        []routingstore.Route
	Providers     []routingstore.Provider
	PolicyBundles []policystore.Bundle
}
