package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// ServerOptions configures the control API HTTP server.
type ServerOptions struct {
	Version        string
	Store          PolicyStore
	Notifier       ActivationNotifier
	ConfigHealth   ConfigHealth
	AuditHealth    AuditHealth
	SpoolStatus    SpoolStatusProvider
	TrafficReader  TrafficReader
	MetricsHandler http.Handler
	HTTPMiddleware func(http.Handler) http.Handler
	BearerToken    string
}

// NewServer builds an HTTP handler for the control API.
func NewServer(opts ServerOptions) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	if opts.HTTPMiddleware != nil {
		r.Use(opts.HTTPMiddleware)
	}
	if opts.BearerToken != "" {
		r.Use(bearerTokenMiddleware(opts.BearerToken))
	}

	policies := newPolicyController(opts.Store, opts.Notifier)

	r.Get("/api/healthz", healthHandler(opts.Version, opts.ConfigHealth, opts.AuditHealth, opts.SpoolStatus))
	r.Get("/api/console/summary", consoleSummaryHandler(opts.Version, opts.Store, opts.SpoolStatus, opts.TrafficReader))
	r.Get("/api/config/active", policies.configStatus)
	r.Get("/api/policy-bundles", policies.listPolicyBundles)
	r.Post("/api/policy-bundles", policies.createPolicyBundle)
	r.Post("/api/policy-bundles/validate", policies.validatePolicyBundle)
	r.Post("/api/policy-bundles/activate", policies.activatePolicyBundles)
	r.Get("/api/model-mappings", policies.listModelMappings)
	r.Put("/api/model-mappings/{id}", policies.putModelMapping)
	r.Get("/api/verdict-providers", policies.listVerdictProviders)
	r.Put("/api/verdict-providers/{id}", policies.putVerdictProvider)
	r.Get("/api/gateway-clients", policies.listGatewayClients)
	r.Post("/api/gateway-clients", policies.createGatewayClient)
	r.Patch("/api/gateway-clients/{client_id}", policies.patchGatewayClient)
	r.Post("/api/gateway-clients/{client_id}/revoke", policies.revokeGatewayClient)
	r.Get("/api/gateway-limits/routes", policies.listRouteLimits)
	r.Put("/api/gateway-limits/routes/{route_key}", policies.putRouteLimit)
	r.Get("/api/gateway-limits/clients/{client_id}/routes", policies.listClientRouteLimits)
	r.Put("/api/gateway-limits/clients/{client_id}/routes/{route_key}", policies.putClientRouteLimit)
	r.Delete("/api/gateway-limits/clients/{client_id}/routes/{route_key}", policies.deleteClientRouteLimit)
	r.Get("/api/traffic/events", trafficEventsHandler(opts.TrafficReader))
	r.Get("/api/traffic/spool", trafficSpoolHandler(opts.SpoolStatus))
	r.Get("/api/policies/bundles", policies.listPolicyBundles)
	r.Post("/api/policies/bundles", policies.createPolicyBundle)
	r.Post("/api/policies/bundles/validate", policies.validatePolicyBundle)
	r.Post("/api/policies/bundles/activate", policies.activatePolicyBundles)
	r.Get("/api/routing/model-mappings", policies.listModelMappings)
	r.Put("/api/routing/model-mappings/{id}", policies.putModelMapping)
	r.Get("/api/providers/verdict", policies.listVerdictProviders)
	r.Put("/api/providers/verdict/{id}", policies.putVerdictProvider)
	r.Get("/api/storage/event-spool", trafficSpoolHandler(opts.SpoolStatus))
	if opts.MetricsHandler != nil {
		r.Handle("/metrics", opts.MetricsHandler)
	}
	r.Post("/api/tools/regex-test", policies.regexTest)
	r.Post("/api/tools/policy-dry-run", policies.policyDryRun)

	return r
}

func bearerTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if controlPathAllowsAnonymous(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+token {
				writeError(w, http.StatusUnauthorized, "unauthorized", "control API bearer token is required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func controlPathAllowsAnonymous(path string) bool {
	return path == "/api/healthz" || path == "/metrics"
}
