package postgres

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresStatusRepository interface {
	LoadActiveConfigSnapshot(context.Context) (configSnapshot, error)
}

type postgresRoutingRepository interface {
	ListRoutes(context.Context) ([]routingstore.Route, error)
	ListProviders(context.Context) ([]routingstore.Provider, error)
	ListModelMappings(context.Context) ([]routingstore.ModelMapping, error)
	UpsertModelMapping(context.Context, routingstore.ModelMapping) error
	ListVerdictProviders(context.Context) ([]routingstore.VerdictProvider, error)
	UpsertVerdictProvider(context.Context, routingstore.VerdictProvider) error
}

type postgresPolicyBundleRepository interface {
	ListPolicyBundles(context.Context) ([]policystore.Bundle, error)
	UpsertPolicyBundle(context.Context, policystore.Bundle) error
	ActivatePolicyBundles(context.Context, policystore.ActivationRequest) (policystore.ActivationResult, error)
}

type postgresClientRepository interface {
	ListGatewayClients(context.Context) ([]clientstore.GatewayClient, error)
	CreateGatewayClient(context.Context, clientstore.GatewayClient) error
	UpsertGatewayClient(context.Context, clientstore.GatewayClient) error
	RevokeGatewayClient(context.Context, string) error
}

type postgresLimitRepository interface {
	ListRouteLimitPolicies(context.Context) ([]limitstore.RoutePolicy, error)
	UpsertRouteLimitPolicy(context.Context, limitstore.RoutePolicy) error
	ListClientRouteLimitOverrides(context.Context, string) ([]limitstore.ClientRouteOverride, error)
	UpsertClientRouteLimitOverride(context.Context, limitstore.ClientRouteOverride) error
	DeleteClientRouteLimitOverride(context.Context, string, string) error
}

type postgresPolicyRepository interface {
	postgresStatusRepository
	postgresRoutingRepository
	postgresPolicyBundleRepository
	postgresClientRepository
	postgresLimitRepository
}

type postgresPolicyStore struct {
	repo postgresPolicyRepository
}

var (
	errGatewayClientNotFound      = appcontrol.ErrGatewayClientNotFound
	errGatewayClientAlreadyExists = appcontrol.ErrGatewayClientAlreadyExists
)

// NewPolicyStore adapts the PostgreSQL config repository to the control application ports.
func NewPolicyStore(repo postgresPolicyRepository) appcontrol.Repository {
	return postgresPolicyStore{repo: repo}
}

// NewStore creates the control-plane PostgreSQL persistence adapter.
func NewStore(pool *pgxpool.Pool) appcontrol.Repository {
	return NewPolicyStore(newConfigBackedRepository(configstore.NewRevisionUnitOfWork(pool)))
}
