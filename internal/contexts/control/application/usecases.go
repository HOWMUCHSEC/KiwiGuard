package control

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// ServiceOptions supplies the repositories and side effects used by control-plane use cases.
type ServiceOptions struct {
	Repository Repository
	Notifier   ActivationNotifier
}

// Service coordinates control-plane configuration use cases.
type Service struct {
	statusRepository          StatusRepository
	policyBundleRepository    PolicyBundleRepository
	modelMappingRepository    ModelMappingRepository
	verdictProviderRepository VerdictProviderRepository
	gatewayClientRepository   GatewayClientRepository
	routeLimitRepository      RouteLimitRepository
	notifier                  ActivationNotifier
}

// NewService assembles the control-plane application service from repository and runtime ports.
func NewService(opts ServiceOptions) *Service {
	repository := opts.Repository
	if isNil(repository) {
		repository = NewMemoryRepository()
	}
	return &Service{
		statusRepository:          repository,
		policyBundleRepository:    repository,
		modelMappingRepository:    repository,
		verdictProviderRepository: repository,
		gatewayClientRepository:   repository,
		routeLimitRepository:      repository,
		notifier:                  opts.Notifier,
	}
}

// ConfigStatus reads the bundle set and snapshot hash currently active in control storage.
func (s *Service) ConfigStatus(ctx context.Context) (ConfigStatus, error) {
	return s.statusRepository.ConfigStatus(ctx)
}

// ListPolicyBundles reads all policy bundles available to the control plane.
func (s *Service) ListPolicyBundles(ctx context.Context) ([]PolicyBundle, error) {
	return s.policyBundleRepository.ListPolicyBundles(ctx)
}

// CreatePolicyBundle compiles a bundle before persisting it so invalid policy never enters storage.
func (s *Service) CreatePolicyBundle(ctx context.Context, bundle PolicyBundle) error {
	if _, err := compilePolicyBundle(bundle); err != nil {
		return invalidInput(err)
	}
	return s.policyBundleRepository.CreatePolicyBundle(ctx, bundle)
}

// ValidatePolicyBundle validates a policy bundle without persisting it.
func (s *Service) ValidatePolicyBundle(bundle PolicyBundle) (string, error) {
	snapshot, err := compilePolicyBundle(bundle)
	if err != nil {
		return "", err
	}
	return snapshot.Hash(), nil
}

// PolicyDryRun evaluates one policy bundle against sample traffic without persisting it.
func (s *Service) PolicyDryRun(request PolicyDryRunRequest) (PolicyDryRunResponse, error) {
	if !validDirection(request.Direction) {
		return PolicyDryRunResponse{}, invalidInput(fmt.Errorf("direction must be input or output"))
	}
	snapshot, err := compilePolicyBundle(request.Bundle)
	if err != nil {
		return PolicyDryRunResponse{}, invalidInput(err)
	}
	decision := snapshot.Evaluate(policy.EvaluationRequest{
		RouteKey:  request.RouteKey,
		Provider:  request.Provider,
		Model:     request.Model,
		Direction: detection.Direction(request.Direction),
		Text:      request.Text,
	})
	return PolicyDryRunResponse{Decision: decision}, nil
}

// ActivatePolicyBundles activates policy bundles and emits a best-effort notification.
func (s *Service) ActivatePolicyBundles(ctx context.Context, request PolicyActivationRequest) (PolicyActivationResponse, error) {
	snapshotHash, err := s.activationSnapshotHash(ctx, request.Keys)
	if err != nil {
		return PolicyActivationResponse{}, err
	}
	request.SnapshotHash = snapshotHash
	response, err := s.policyBundleRepository.ActivatePolicyBundles(ctx, request)
	if err != nil {
		return PolicyActivationResponse{}, err
	}
	if !isNil(s.notifier) {
		if err := s.notifier.NotifyConfigActivated(ctx, response.RevisionNumber); err != nil {
			response.NotificationError = "notify config activation failed"
		}
	}
	return response, nil
}

func (s *Service) activationSnapshotHash(ctx context.Context, keys []string) (string, error) {
	bundles, err := s.policyBundleRepository.ListPolicyBundles(ctx)
	if err != nil {
		return "", err
	}
	byKey := make(map[string]PolicyBundle, len(bundles))
	for _, bundle := range bundles {
		byKey[bundle.Key] = bundle
	}
	selected := make([]policy.Bundle, 0, len(keys))
	for _, key := range keys {
		bundle, ok := byKey[key]
		if !ok {
			return "", fmt.Errorf("policy bundle not found")
		}
		if err := bundle.Validate(); err != nil {
			return "", invalidInput(err)
		}
		selected = append(selected, bundle.ToPolicyBundle())
	}
	snapshot, err := policy.CompileSnapshot(selected)
	if err != nil {
		return "", invalidInput(err)
	}
	return snapshot.Hash(), nil
}

// ListModelMappings reads every configured model mapping from control storage.
func (s *Service) ListModelMappings(ctx context.Context) ([]ModelMapping, error) {
	return s.modelMappingRepository.ListModelMappings(ctx)
}

// PutModelMapping persists one model mapping after transport-layer validation.
func (s *Service) PutModelMapping(ctx context.Context, mapping ModelMapping) error {
	return s.modelMappingRepository.PutModelMapping(ctx, mapping)
}

// ListVerdictProviders reads every configured verdict provider from control storage.
func (s *Service) ListVerdictProviders(ctx context.Context) ([]VerdictProvider, error) {
	return s.verdictProviderRepository.ListVerdictProviders(ctx)
}

// PutVerdictProvider normalizes and persists one verdict provider configuration.
func (s *Service) PutVerdictProvider(ctx context.Context, provider VerdictProvider) error {
	provider, err := normalizeVerdictProvider(provider)
	if err != nil {
		return invalidInput(err)
	}
	return s.verdictProviderRepository.PutVerdictProvider(ctx, provider)
}

func compilePolicyBundle(bundle PolicyBundle) (*policy.Snapshot, error) {
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	return policy.CompileSnapshot([]policy.Bundle{bundle.ToPolicyBundle()})
}

func normalizeVerdictProvider(provider VerdictProvider) (VerdictProvider, error) {
	if provider.Mode != "" && provider.Mode != "inline" {
		return VerdictProvider{}, errors.New("verdict provider mode must be inline")
	}
	provider.Mode = "inline"
	return provider, nil
}

// ListGatewayClients reads all gateway clients visible to the control plane.
func (s *Service) ListGatewayClients(ctx context.Context) ([]GatewayClient, error) {
	return s.gatewayClientRepository.ListGatewayClients(ctx)
}

// CreateGatewayClient validates client metadata before minting and persisting a new key.
func (s *Service) CreateGatewayClient(ctx context.Context, request CreateGatewayClientRequest) (CreateGatewayClientResponse, error) {
	if err := request.NormalizeAndValidate(); err != nil {
		return CreateGatewayClientResponse{}, invalidInput(err)
	}
	return s.gatewayClientRepository.CreateGatewayClient(ctx, request)
}

// PatchGatewayClient validates client metadata before applying an update.
func (s *Service) PatchGatewayClient(ctx context.Context, client GatewayClient) (GatewayClient, error) {
	if err := client.Validate(); err != nil {
		return GatewayClient{}, invalidInput(err)
	}
	return s.gatewayClientRepository.PatchGatewayClient(ctx, client)
}

// RevokeGatewayClient permanently transitions a gateway client into the revoked state.
func (s *Service) RevokeGatewayClient(ctx context.Context, clientID string) (GatewayClient, error) {
	if clientID == "" {
		return GatewayClient{}, invalidInput(fmt.Errorf("client_id is required"))
	}
	return s.gatewayClientRepository.RevokeGatewayClient(ctx, clientID)
}

// ListRouteLimits reads the default gateway limit policy for every configured route.
func (s *Service) ListRouteLimits(ctx context.Context) ([]RouteLimit, error) {
	return s.routeLimitRepository.ListRouteLimits(ctx)
}

// PutRouteLimit validates and persists one default route limit policy.
func (s *Service) PutRouteLimit(ctx context.Context, limit RouteLimit) (RouteLimit, error) {
	if err := limit.Validate(); err != nil {
		return RouteLimit{}, invalidInput(err)
	}
	return s.routeLimitRepository.PutRouteLimit(ctx, limit)
}

// ListClientRouteLimits reads all per-client route limit overrides for one client.
func (s *Service) ListClientRouteLimits(ctx context.Context, clientID string) ([]ClientRouteLimit, error) {
	if clientID == "" {
		return nil, invalidInput(fmt.Errorf("client_id is required"))
	}
	return s.routeLimitRepository.ListClientRouteLimits(ctx, clientID)
}

// PutClientRouteLimit validates and persists one per-client route limit override.
func (s *Service) PutClientRouteLimit(ctx context.Context, limit ClientRouteLimit) (ClientRouteLimit, error) {
	if err := limit.Validate(); err != nil {
		return ClientRouteLimit{}, invalidInput(err)
	}
	return s.routeLimitRepository.PutClientRouteLimit(ctx, limit)
}

// DeleteClientRouteLimit removes one per-client route limit override.
func (s *Service) DeleteClientRouteLimit(ctx context.Context, clientID string, routeKey string) error {
	if clientID == "" {
		return invalidInput(fmt.Errorf("client_id is required"))
	}
	if routeKey == "" {
		return invalidInput(fmt.Errorf("route key is required"))
	}
	return s.routeLimitRepository.DeleteClientRouteLimit(ctx, clientID, routeKey)
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflectValue.IsNil()
	default:
		return false
	}
}
