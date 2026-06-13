package control

import (
	"context"
	"fmt"
	"sync"

	clients "github.com/howmuchsec/kiwiguard/internal/contexts/clients/domain"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// MemoryRepository keeps control-plane configuration in memory for tests and ephemeral deployments.
type MemoryRepository struct {
	mu                sync.RWMutex
	bundles           map[string]PolicyBundle
	activeBundleKeys  []string
	modelMappings     map[string]ModelMapping
	verdictProviders  map[string]VerdictProvider
	gatewayClients    map[string]GatewayClient
	routeLimits       map[string]RouteLimit
	clientRouteLimits map[string]map[string]ClientRouteLimit
	nextClientNumber  int
}

// NewMemoryRepository initializes the in-memory control-plane repository used by tests and ephemeral runs.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		bundles:           make(map[string]PolicyBundle),
		modelMappings:     make(map[string]ModelMapping),
		verdictProviders:  make(map[string]VerdictProvider),
		gatewayClients:    make(map[string]GatewayClient),
		routeLimits:       make(map[string]RouteLimit),
		clientRouteLimits: make(map[string]map[string]ClientRouteLimit),
	}
}

// ConfigStatus derives the active bundle hash from the in-memory bundle set.
func (r *MemoryRepository) ConfigStatus(context.Context) (ConfigStatus, error) {
	r.mu.RLock()
	bundles := r.activeBundlesLocked()
	keys := append([]string(nil), r.activeBundleKeys...)
	r.mu.RUnlock()

	snapshot, _ := policy.CompileSnapshot(bundles)
	hash := ""
	if snapshot != nil {
		hash = snapshot.Hash()
	}
	return ConfigStatus{
		ActivePolicyBundleKeys: keys,
		PolicySnapshotHash:     hash,
	}, nil
}

// ListPolicyBundles returns every bundle currently stored in memory.
func (r *MemoryRepository) ListPolicyBundles(context.Context) ([]PolicyBundle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]PolicyBundle, 0, len(r.bundles))
	for _, bundle := range r.bundles {
		items = append(items, bundle)
	}
	return items, nil
}

// CreatePolicyBundle upserts one policy bundle into the in-memory store.
func (r *MemoryRepository) CreatePolicyBundle(_ context.Context, bundle PolicyBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.bundles[bundle.Key] = bundle
	return nil
}

// ActivatePolicyBundles switches the active bundle set after verifying every requested key exists.
func (r *MemoryRepository) ActivatePolicyBundles(_ context.Context, request PolicyActivationRequest) (PolicyActivationResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, key := range request.Keys {
		if _, ok := r.bundles[key]; !ok {
			return PolicyActivationResponse{}, fmt.Errorf("policy bundle not found")
		}
	}
	r.activeBundleKeys = append([]string(nil), request.Keys...)

	return PolicyActivationResponse{ActiveKeys: request.Keys, Hash: request.SnapshotHash}, nil
}

// ListModelMappings returns every model mapping currently stored in memory.
func (r *MemoryRepository) ListModelMappings(context.Context) ([]ModelMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]ModelMapping, 0, len(r.modelMappings))
	for _, mapping := range r.modelMappings {
		items = append(items, mapping)
	}
	return items, nil
}

// PutModelMapping upserts one model mapping into the in-memory store.
func (r *MemoryRepository) PutModelMapping(_ context.Context, mapping ModelMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.modelMappings[mapping.ID] = mapping
	return nil
}

// ListVerdictProviders returns every verdict provider currently stored in memory.
func (r *MemoryRepository) ListVerdictProviders(context.Context) ([]VerdictProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]VerdictProvider, 0, len(r.verdictProviders))
	for _, provider := range r.verdictProviders {
		items = append(items, provider)
	}
	return items, nil
}

// PutVerdictProvider upserts one verdict provider into the in-memory store.
func (r *MemoryRepository) PutVerdictProvider(_ context.Context, provider VerdictProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.verdictProviders[provider.ID] = provider
	return nil
}

// ListGatewayClients returns every gateway client currently stored in memory.
func (r *MemoryRepository) ListGatewayClients(context.Context) ([]GatewayClient, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]GatewayClient, 0, len(r.gatewayClients))
	for _, client := range r.gatewayClients {
		items = append(items, client)
	}
	return items, nil
}

// CreateGatewayClient mints one plaintext key and persists only the client-facing metadata.
func (r *MemoryRepository) CreateGatewayClient(_ context.Context, request CreateGatewayClientRequest) (CreateGatewayClientResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextClientNumber++
	if request.ID == "" {
		request.ID = fmt.Sprintf("client-%d", r.nextClientNumber)
	}
	if _, exists := r.gatewayClients[request.ID]; exists {
		return CreateGatewayClientResponse{}, ErrGatewayClientAlreadyExists
	}
	key, material, err := clients.GenerateKey(request.ID)
	if err != nil {
		return CreateGatewayClientResponse{}, err
	}
	client := GatewayClient{
		ID:        request.ID,
		Name:      request.Name,
		Status:    request.Status,
		KeyPrefix: material.Prefix,
		Notes:     request.Notes,
	}
	r.gatewayClients[client.ID] = client
	return CreateGatewayClientResponse{Client: client, Key: key}, nil
}

// PatchGatewayClient updates mutable client metadata while preserving revoked state and key prefix.
func (r *MemoryRepository) PatchGatewayClient(_ context.Context, client GatewayClient) (GatewayClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.gatewayClients[client.ID]
	if !ok {
		return GatewayClient{}, ErrGatewayClientNotFound
	}
	if existing.KeyPrefix != "" && client.KeyPrefix == "" {
		client.KeyPrefix = existing.KeyPrefix
	}
	if existing.Status == string(clients.StatusRevoked) {
		client.Status = existing.Status
	}
	r.gatewayClients[client.ID] = client
	return client, nil
}

// RevokeGatewayClient transitions one in-memory client record into the revoked state.
func (r *MemoryRepository) RevokeGatewayClient(_ context.Context, clientID string) (GatewayClient, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	client, ok := r.gatewayClients[clientID]
	if !ok {
		return GatewayClient{}, ErrGatewayClientNotFound
	}
	client.ID = clientID
	client.Status = string(clients.StatusRevoked)
	r.gatewayClients[clientID] = client
	return client, nil
}

// ListRouteLimits exposes the default route-limit set currently held in memory.
func (r *MemoryRepository) ListRouteLimits(context.Context) ([]RouteLimit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]RouteLimit, 0, len(r.routeLimits))
	for _, limit := range r.routeLimits {
		items = append(items, limit)
	}
	return items, nil
}

// PutRouteLimit upserts one default route limit into the in-memory store.
func (r *MemoryRepository) PutRouteLimit(_ context.Context, limit RouteLimit) (RouteLimit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.routeLimits[limit.RouteKey] = limit
	return limit, nil
}

// ListClientRouteLimits returns every stored route override for one known client.
func (r *MemoryRepository) ListClientRouteLimits(_ context.Context, clientID string) ([]ClientRouteLimit, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.gatewayClients[clientID]; !ok {
		return nil, ErrGatewayClientNotFound
	}
	limits := r.clientRouteLimits[clientID]
	items := make([]ClientRouteLimit, 0, len(limits))
	for _, limit := range limits {
		items = append(items, limit)
	}
	return items, nil
}

// PutClientRouteLimit upserts one per-client route limit override in the in-memory store.
func (r *MemoryRepository) PutClientRouteLimit(_ context.Context, limit ClientRouteLimit) (ClientRouteLimit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.gatewayClients[limit.ClientID]; !ok {
		return ClientRouteLimit{}, ErrGatewayClientNotFound
	}
	if r.clientRouteLimits[limit.ClientID] == nil {
		r.clientRouteLimits[limit.ClientID] = make(map[string]ClientRouteLimit)
	}
	r.clientRouteLimits[limit.ClientID][limit.RouteKey] = limit
	return limit, nil
}

// DeleteClientRouteLimit removes one stored per-client route limit override.
func (r *MemoryRepository) DeleteClientRouteLimit(_ context.Context, clientID string, routeKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.gatewayClients[clientID]; !ok {
		return ErrGatewayClientNotFound
	}
	delete(r.clientRouteLimits[clientID], routeKey)
	return nil
}

func (r *MemoryRepository) activeBundlesLocked() []policy.Bundle {
	bundles := make([]policy.Bundle, 0, len(r.activeBundleKeys))
	for _, key := range r.activeBundleKeys {
		bundle, ok := r.bundles[key]
		if !ok {
			continue
		}
		bundles = append(bundles, bundle.ToPolicyBundle())
	}
	return bundles
}
