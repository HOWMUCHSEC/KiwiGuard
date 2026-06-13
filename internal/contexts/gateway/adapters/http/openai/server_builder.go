package openai

import (
	"net/http"
	"sync"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	verdict "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

type providerServer struct {
	provider ConfigProvider
	mu       sync.Mutex
	revision int64
	server   *server
}

// NewServer builds the OpenAI-compatible gateway HTTP handler with a static transport config.
func NewServer(cfg Config) http.Handler {
	return newServer(cfg)
}

// NewServerWithProvider builds the OpenAI-compatible gateway handler around a runtime config provider.
func NewServerWithProvider(provider ConfigProvider) http.Handler {
	if static, ok := provider.(StaticConfigProvider); ok {
		return newServer(static.Config)
	}
	if static, ok := provider.(*StaticConfigProvider); ok && static != nil {
		return newServer(static.Config)
	}
	return &providerServer{provider: provider}
}

func (s *providerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cfg := Config{}
	if s.provider != nil {
		cfg = s.provider.CurrentConfig()
	}
	s.currentServer(cfg).ServeHTTP(w, r)
}

func (s *providerServer) currentServer(cfg Config) *server {
	if cfg.ConfigRevisionNumber == 0 {
		return newServer(cfg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil && s.revision == cfg.ConfigRevisionNumber {
		return s.server
	}

	current := newServer(cfg)
	s.revision = cfg.ConfigRevisionNumber
	s.server = current
	return current
}

func newServer(cfg Config) *server {
	maxBodyBytes := cfg.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	upstreamTimeout := cfg.UpstreamTimeout
	if upstreamTimeout <= 0 {
		upstreamTimeout = defaultUpstreamTimeout
	}
	verdictTimeout := cfg.VerdictTimeout
	if verdictTimeout <= 0 {
		verdictTimeout = defaultVerdictTimeout
	}

	s := &server{
		configRevisionNumber: cfg.ConfigRevisionNumber,
		maxBodyBytes:         maxBodyBytes,
		upstreamTimeout:      upstreamTimeout,
		verdictTimeout:       verdictTimeout,
		routes:               make(map[routeKey]Route, len(cfg.Routes)),
		providers:            make(map[string]providerConfig, len(cfg.Providers)),
		snapshot:             cfg.Snapshot,
		verdict:              cfg.VerdictProvider,
		verdictProviders:     cloneVerdictProviders(cfg.VerdictProviders),
		events:               cfg.EventWriter,
		auditGate:            cfg.AuditGate,
		rawCapturePolicies:   append([]RawCapturePolicy(nil), cfg.RawCapturePolicies...),
		clients:              newClientRegistry(cfg.Clients),
		limitResolver:        newLimitResolver(cfg.Routes, cfg.RouteLimits, cfg.ClientRouteLimitOverrides),
		limitState:           newLimitState(),
		lifecycle:            appgateway.LifecycleUseCase{},
	}
	for _, route := range cfg.Routes {
		method := route.Method
		if method == "" {
			method = http.MethodPost
		}
		if route.Execution == "" {
			route.Execution = ExecutionInline
		}
		s.routes[routeKey{method: method, path: route.Path}] = route
	}
	for _, provider := range cfg.Providers {
		client := provider.Client
		if client == nil {
			client = &http.Client{Timeout: provider.Timeout}
		}
		s.providers[provider.Key] = providerConfig{provider: provider, client: client}
	}
	return s
}

func cloneVerdictProviders(providers map[string]verdict.Provider) map[string]verdict.Provider {
	if len(providers) == 0 {
		return nil
	}
	cloned := make(map[string]verdict.Provider, len(providers))
	for key, provider := range providers {
		cloned[key] = provider
	}
	return cloned
}
