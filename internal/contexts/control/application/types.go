// Package control contains control-plane configuration application use cases.
package control

import (
	"context"
	"errors"
	"fmt"
	"strings"

	clients "github.com/howmuchsec/kiwiguard/internal/contexts/clients/domain"
	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

var (
	// ErrGatewayClientNotFound reports that a gateway client does not exist.
	ErrGatewayClientNotFound = errors.New("gateway client not found")
	// ErrGatewayClientAlreadyExists reports that a gateway client external ID is already in use.
	ErrGatewayClientAlreadyExists = errors.New("gateway client already exists")
	// ErrInvalidInput reports request input that fails application validation.
	ErrInvalidInput = errors.New("invalid input")
)

type validationError struct {
	err error
}

func (e validationError) Error() string {
	return e.err.Error()
}

func (e validationError) Unwrap() error {
	return e.err
}

func (e validationError) Is(target error) bool {
	return target == ErrInvalidInput
}

func invalidInput(err error) error {
	if err == nil {
		return nil
	}
	return validationError{err: err}
}

// StatusRepository exposes read access to the currently active configuration snapshot.
type StatusRepository interface {
	ConfigStatus(context.Context) (ConfigStatus, error)
}

// PolicyBundleRepository manages policy bundle drafts and activation requests.
type PolicyBundleRepository interface {
	ListPolicyBundles(context.Context) ([]PolicyBundle, error)
	CreatePolicyBundle(context.Context, PolicyBundle) error
	ActivatePolicyBundles(context.Context, PolicyActivationRequest) (PolicyActivationResponse, error)
}

// ModelMappingRepository manages route/provider model mapping configuration.
type ModelMappingRepository interface {
	ListModelMappings(context.Context) ([]ModelMapping, error)
	PutModelMapping(context.Context, ModelMapping) error
}

// VerdictProviderRepository manages vertical verdict provider configuration.
type VerdictProviderRepository interface {
	ListVerdictProviders(context.Context) ([]VerdictProvider, error)
	PutVerdictProvider(context.Context, VerdictProvider) error
}

// GatewayClientRepository manages gateway client lifecycle and key material.
type GatewayClientRepository interface {
	ListGatewayClients(context.Context) ([]GatewayClient, error)
	CreateGatewayClient(context.Context, CreateGatewayClientRequest) (CreateGatewayClientResponse, error)
	PatchGatewayClient(context.Context, GatewayClient) (GatewayClient, error)
	RevokeGatewayClient(context.Context, string) (GatewayClient, error)
}

// RouteLimitRepository manages default and client-specific gateway limit policies.
type RouteLimitRepository interface {
	ListRouteLimits(context.Context) ([]RouteLimit, error)
	PutRouteLimit(context.Context, RouteLimit) (RouteLimit, error)
	ListClientRouteLimits(context.Context, string) ([]ClientRouteLimit, error)
	PutClientRouteLimit(context.Context, ClientRouteLimit) (ClientRouteLimit, error)
	DeleteClientRouteLimit(context.Context, string, string) error
}

// Repository groups the persistence ports required by the control-plane application service.
type Repository interface {
	StatusRepository
	PolicyBundleRepository
	ModelMappingRepository
	VerdictProviderRepository
	GatewayClientRepository
	RouteLimitRepository
}

// ActivationNotifier publishes successful config activation events.
type ActivationNotifier interface {
	NotifyConfigActivated(context.Context, int64) error
}

// ConfigStatus reports the bundle set and snapshot hash currently active at runtime.
type ConfigStatus struct {
	ActivePolicyBundleKeys []string
	PolicySnapshotHash     string
}

// PolicyBundle is the control-plane contract for one managed policy bundle.
type PolicyBundle struct {
	Key           string
	Version       string
	Source        string
	DefaultAction string
	Detectors     []Detector
	Rules         []Rule
}

// Detector is the control-plane contract for one detector definition inside a bundle.
type Detector struct {
	Key        string
	Kind       string
	Pattern    string
	Categories []string
}

// Rule is the control-plane contract for one policy rule inside a bundle.
type Rule struct {
	Key          string
	Enabled      bool
	Severity     string
	Action       string
	DetectorKeys []string
	Scope        Scope
}

// Scope narrows a rule to selected route, provider, model, or direction dimensions.
type Scope struct {
	RouteKey  string
	Provider  string
	Model     string
	Direction string
}

// PolicyActivationRequest asks the repository to promote a new active bundle set.
type PolicyActivationRequest struct {
	Keys         []string
	Reason       string
	SnapshotHash string
}

// PolicyActivationResponse reports the revision and bundle set that became active.
type PolicyActivationResponse struct {
	ActiveKeys        []string
	Hash              string
	NotificationError string
	RevisionNumber    int64
}

// PolicyDryRunRequest supplies representative traffic for bundle validation without activation.
type PolicyDryRunRequest struct {
	RouteKey  string
	Provider  string
	Model     string
	Direction string
	Text      string
	Bundle    PolicyBundle
}

// PolicyDryRunResponse carries the policy decision produced by a dry run.
type PolicyDryRunResponse struct {
	Decision policy.Decision
}

// ModelMapping is the control-plane contract for a public-to-upstream model mapping.
type ModelMapping struct {
	ID       string
	RouteKey string
	Provider string
	Model    string
	Enabled  bool
}

// VerdictProvider is the control-plane contract for one external verdict provider.
type VerdictProvider struct {
	ID            string
	Name          string
	Endpoint      string
	CredentialRef string
	Mode          string
	Enabled       bool
}

// GatewayClient identifies a caller authorized to send traffic through the gateway.
type GatewayClient struct {
	ID        string
	Name      string
	Status    string
	KeyPrefix string
	Notes     string
}

// CreateGatewayClientRequest carries the metadata required to mint a new gateway client key.
type CreateGatewayClientRequest struct {
	ID     string
	Name   string
	Status string
	Notes  string
}

// CreateGatewayClientResponse returns the persisted client plus its one-time plaintext key.
type CreateGatewayClientResponse struct {
	Client GatewayClient
	Key    string
}

// RouteLimit defines the default request, concurrency, and body limits for one route.
type RouteLimit struct {
	RouteKey              string
	RequestsPerWindow     int
	WindowSeconds         int
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}

// ClientRouteLimit defines per-client overrides for one route's default gateway limits.
type ClientRouteLimit struct {
	ClientID              string
	RouteKey              string
	RequestsPerWindow     int
	WindowSeconds         int
	MaxConcurrentRequests int
	MaxBodyBytes          int64
	Enabled               bool
}

// Validate validates a policy bundle before persistence or compilation.
func (b PolicyBundle) Validate() error {
	if !validSource(b.Source) {
		return fmt.Errorf("source %q is invalid", b.Source)
	}
	if !validAction(b.DefaultAction) {
		return fmt.Errorf("default action %q is invalid", b.DefaultAction)
	}
	for _, detector := range b.Detectors {
		if !validDetectorKind(detector.Kind) {
			return fmt.Errorf("detector %s kind %q is invalid", detector.Key, detector.Kind)
		}
	}
	for _, rule := range b.Rules {
		if !validSeverity(rule.Severity) {
			return fmt.Errorf("rule %s severity %q is invalid", rule.Key, rule.Severity)
		}
		if !validAction(rule.Action) {
			return fmt.Errorf("rule %s action %q is invalid", rule.Key, rule.Action)
		}
		if rule.Scope.Direction != "" && !validDirection(rule.Scope.Direction) {
			return fmt.Errorf("rule %s scope direction %q is invalid", rule.Key, rule.Scope.Direction)
		}
	}
	return nil
}

// ToPolicyBundle converts control-plane input into a policy domain bundle.
func (b PolicyBundle) ToPolicyBundle() policy.Bundle {
	detectorDefs := make([]detection.Definition, 0, len(b.Detectors))
	for _, detector := range b.Detectors {
		detectorDefs = append(detectorDefs, detection.Definition{
			Key:        detector.Key,
			Kind:       detection.Kind(detector.Kind),
			Pattern:    detector.Pattern,
			Categories: detector.Categories,
		})
	}

	rules := make([]policy.Rule, 0, len(b.Rules))
	for _, rule := range b.Rules {
		rules = append(rules, policy.Rule{
			Key:          rule.Key,
			Enabled:      rule.Enabled,
			Severity:     policy.Severity(rule.Severity),
			Action:       policy.Action(rule.Action),
			DetectorKeys: rule.DetectorKeys,
			Scope: policy.Scope{
				RouteKey:  rule.Scope.RouteKey,
				Provider:  rule.Scope.Provider,
				Model:     rule.Scope.Model,
				Direction: detection.Direction(rule.Scope.Direction),
			},
		})
	}

	return policy.Bundle{
		Key:           b.Key,
		Version:       b.Version,
		Source:        policy.Source(b.Source),
		DefaultAction: policy.Action(b.DefaultAction),
		Detectors:     detectorDefs,
		Rules:         rules,
	}
}

// NormalizeAndValidate prepares and validates a gateway client creation request.
func (r *CreateGatewayClientRequest) NormalizeAndValidate() error {
	r.ID = strings.TrimSpace(r.ID)
	r.Name = strings.TrimSpace(r.Name)
	if r.Status == "" {
		r.Status = string(clients.StatusEnabled)
	}
	r.Status = strings.TrimSpace(r.Status)
	if r.Name == "" {
		return fmt.Errorf("client name is required")
	}
	if !validGatewayClientStatus(r.Status) {
		return fmt.Errorf("client status must be enabled, disabled, or revoked")
	}
	return nil
}

// Validate validates a gateway client update.
func (c GatewayClient) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("client name is required")
	}
	if !validGatewayClientStatus(c.Status) {
		return fmt.Errorf("client status must be enabled, disabled, or revoked")
	}
	return nil
}

// Validate validates a route limit.
func (l RouteLimit) Validate() error {
	if strings.TrimSpace(l.RouteKey) == "" {
		return fmt.Errorf("route key is required")
	}
	if !validLimitValues(l.RequestsPerWindow, l.WindowSeconds, l.MaxConcurrentRequests, l.MaxBodyBytes) {
		return fmt.Errorf("limit values must be greater than zero")
	}
	return nil
}

// Validate validates a client route limit.
func (l ClientRouteLimit) Validate() error {
	if strings.TrimSpace(l.ClientID) == "" {
		return fmt.Errorf("client_id is required")
	}
	if strings.TrimSpace(l.RouteKey) == "" {
		return fmt.Errorf("route key is required")
	}
	if !validLimitValues(l.RequestsPerWindow, l.WindowSeconds, l.MaxConcurrentRequests, l.MaxBodyBytes) {
		return fmt.Errorf("limit values must be greater than zero")
	}
	return nil
}

func validGatewayClientStatus(status string) bool {
	switch clients.Status(status) {
	case clients.StatusEnabled, clients.StatusDisabled, clients.StatusRevoked:
		return true
	default:
		return false
	}
}

func validLimitValues(requestsPerWindow int, windowSeconds int, maxConcurrentRequests int, maxBodyBytes int64) bool {
	return requestsPerWindow > 0 && windowSeconds > 0 && maxConcurrentRequests > 0 && maxBodyBytes > 0
}

func validSource(source string) bool {
	switch policy.Source(source) {
	case policy.SourceBuiltIn, policy.SourceUser, policy.SourceImported:
		return true
	default:
		return false
	}
}

func validAction(action string) bool {
	switch policy.Action(action) {
	case policy.ActionAllow, policy.ActionBlock, policy.ActionRedact, policy.ActionShadowLog:
		return true
	default:
		return false
	}
}

func validSeverity(severity string) bool {
	switch policy.Severity(severity) {
	case policy.SeverityLow, policy.SeverityMedium, policy.SeverityHigh, policy.SeverityCritical:
		return true
	default:
		return false
	}
}

func validDetectorKind(kind string) bool {
	switch detection.Kind(kind) {
	case detection.KindRegex, detection.KindEmail, detection.KindPhone, detection.KindPaymentCard, detection.KindSecret:
		return true
	default:
		return false
	}
}

func validDirection(direction string) bool {
	switch detection.Direction(direction) {
	case detection.DirectionInput, detection.DirectionOutput:
		return true
	default:
		return false
	}
}
