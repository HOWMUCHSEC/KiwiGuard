package control

import (
	"context"
	"errors"
	"testing"
)

func TestCreatePolicyBundleValidatesBeforeRepositoryWrite(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(ServiceOptions{Repository: repo})

	err := service.CreatePolicyBundle(context.Background(), PolicyBundle{
		Key:           "pii",
		Version:       "v1",
		Source:        "user",
		DefaultAction: "allow",
		Detectors: []Detector{{
			Key:     "bad",
			Kind:    "unknown",
			Pattern: ".+",
		}},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreatePolicyBundle() error = %v, want ErrInvalidInput", err)
	}

	bundles, listErr := repo.ListPolicyBundles(context.Background())
	if listErr != nil {
		t.Fatalf("ListPolicyBundles() error = %v", listErr)
	}
	if len(bundles) != 0 {
		t.Fatalf("stored bundles = %+v, want none", bundles)
	}
}

func TestActivatePolicyBundlesRecordsNotificationFailure(t *testing.T) {
	notifier := failingNotifier{err: errors.New("publish failed")}
	service := NewService(ServiceOptions{
		Repository: NewMemoryRepository(),
		Notifier:   notifier,
	})
	bundle := PolicyBundle{
		Key:           "pii",
		Version:       "v1",
		Source:        "user",
		DefaultAction: "allow",
	}
	if err := service.CreatePolicyBundle(context.Background(), bundle); err != nil {
		t.Fatalf("CreatePolicyBundle() error = %v", err)
	}

	response, err := service.ActivatePolicyBundles(context.Background(), PolicyActivationRequest{Keys: []string{"pii"}})
	if err != nil {
		t.Fatalf("ActivatePolicyBundles() error = %v", err)
	}
	if response.NotificationError != "notify config activation failed" {
		t.Fatalf("NotificationError = %q, want notification failure marker", response.NotificationError)
	}
}

func TestActivatePolicyBundlesPropagatesRepositoryFailures(t *testing.T) {
	t.Run("list failure", func(t *testing.T) {
		wantErr := errors.New("list failed")
		repo := &failingPolicyRepository{
			MemoryRepository:     NewMemoryRepository(),
			listPolicyBundlesErr: wantErr,
		}
		service := NewService(ServiceOptions{Repository: repo})

		_, err := service.ActivatePolicyBundles(context.Background(), PolicyActivationRequest{Keys: []string{"pii"}})
		if !errors.Is(err, wantErr) {
			t.Fatalf("ActivatePolicyBundles() error = %v, want %v", err, wantErr)
		}
	})

	t.Run("activate failure", func(t *testing.T) {
		wantErr := errors.New("activate failed")
		repo := &failingPolicyRepository{MemoryRepository: NewMemoryRepository()}
		service := NewService(ServiceOptions{
			Repository: repo,
			Notifier:   &recordingNotifier{},
		})
		if err := service.CreatePolicyBundle(context.Background(), minimalPolicyBundle("pii")); err != nil {
			t.Fatalf("CreatePolicyBundle() error = %v", err)
		}
		repo.activatePolicyBundlesErr = wantErr

		_, err := service.ActivatePolicyBundles(context.Background(), PolicyActivationRequest{Keys: []string{"pii"}})
		if !errors.Is(err, wantErr) {
			t.Fatalf("ActivatePolicyBundles() error = %v, want %v", err, wantErr)
		}
	})
}

func TestActivatePolicyBundlesRejectsInvalidSelections(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.ActivatePolicyBundles(context.Background(), PolicyActivationRequest{Keys: []string{"missing"}})
	if err == nil {
		t.Fatalf("ActivatePolicyBundles() error = nil, want missing bundle error")
	}

	repo := NewMemoryRepository()
	if err := repo.CreatePolicyBundle(context.Background(), PolicyBundle{
		Key:           "bad",
		Version:       "v1",
		Source:        "user",
		DefaultAction: "drop",
	}); err != nil {
		t.Fatalf("CreatePolicyBundle() setup error = %v", err)
	}
	service = NewService(ServiceOptions{Repository: repo})

	_, err = service.ActivatePolicyBundles(context.Background(), PolicyActivationRequest{Keys: []string{"bad"}})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ActivatePolicyBundles() error = %v, want ErrInvalidInput", err)
	}
}

func TestValidatePolicyBundleReturnsHashAndRejectsInvalidBundle(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	hash, err := service.ValidatePolicyBundle(minimalPolicyBundle("pii"))
	if err != nil {
		t.Fatalf("ValidatePolicyBundle() error = %v", err)
	}
	if hash == "" {
		t.Fatalf("ValidatePolicyBundle() hash is empty, want compiled snapshot hash")
	}

	_, err = service.ValidatePolicyBundle(PolicyBundle{
		Key:           "pii",
		Version:       "v1",
		Source:        "external",
		DefaultAction: "allow",
	})
	if err == nil {
		t.Fatalf("ValidatePolicyBundle() error = nil, want invalid source error")
	}
}

func TestCreateGatewayClientNormalizesDefaultsAndDetectsDuplicateIDs(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	created, err := service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: " Acme ",
	})
	if err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}
	if created.Client.Status != "enabled" {
		t.Fatalf("created status = %q, want enabled", created.Client.Status)
	}
	if created.Client.Name != "Acme" {
		t.Fatalf("created name = %q, want trimmed Acme", created.Client.Name)
	}
	if created.Key == "" || created.Client.KeyPrefix == "" {
		t.Fatalf("created key material = %+v, want plaintext key and key prefix", created)
	}

	_, err = service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: "Acme Again",
	})
	if !errors.Is(err, ErrGatewayClientAlreadyExists) {
		t.Fatalf("duplicate CreateGatewayClient() error = %v, want ErrGatewayClientAlreadyExists", err)
	}
}

func TestCreateGatewayClientValidatesBeforeRepositoryWrite(t *testing.T) {
	repo := &recordingRepository{MemoryRepository: NewMemoryRepository()}
	service := NewService(ServiceOptions{Repository: repo})

	_, err := service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: " ",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("CreateGatewayClient() error = %v, want ErrInvalidInput", err)
	}
	if repo.createGatewayClientCalled {
		t.Fatalf("CreateGatewayClient() wrote to repository for invalid request")
	}
}

func TestCreateGatewayClientPropagatesRepositoryFailure(t *testing.T) {
	wantErr := errors.New("store failed")
	repo := &recordingRepository{
		MemoryRepository:       NewMemoryRepository(),
		createGatewayClientErr: wantErr,
	}
	service := NewService(ServiceOptions{Repository: repo})

	_, err := service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: "Acme",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("CreateGatewayClient() error = %v, want %v", err, wantErr)
	}
}

func TestPutVerdictProviderRejectsUnsupportedMode(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	err := service.PutVerdictProvider(context.Background(), VerdictProvider{
		ID:       "sec",
		Name:     "Security model",
		Endpoint: "http://127.0.0.1:9000",
		Mode:     "async",
		Enabled:  true,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutVerdictProvider() error = %v, want ErrInvalidInput", err)
	}
}

func TestPolicyDryRunEvaluatesBundleInApplicationLayer(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	response, err := service.PolicyDryRun(PolicyDryRunRequest{
		RouteKey:  "chat",
		Provider:  "openai",
		Model:     "gpt-test",
		Direction: "input",
		Text:      "contact alice@example.com",
		Bundle: PolicyBundle{
			Key:           "pii",
			Version:       "v1",
			Source:        "user",
			DefaultAction: "allow",
			Detectors: []Detector{{
				Key:     "email",
				Kind:    "email",
				Pattern: "",
			}},
			Rules: []Rule{{
				Key:          "email-block",
				Enabled:      true,
				Severity:     "high",
				Action:       "block",
				DetectorKeys: []string{"email"},
				Scope:        Scope{Direction: "input"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("PolicyDryRun() error = %v", err)
	}
	if response.Decision.Action != "block" {
		t.Fatalf("decision action = %q, want block", response.Decision.Action)
	}
	if len(response.Decision.RuleHits) != 1 || response.Decision.RuleHits[0].RuleKey != "email-block" {
		t.Fatalf("rule hits = %+v, want email-block", response.Decision.RuleHits)
	}
}

func TestPolicyDryRunRejectsInvalidDirection(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PolicyDryRun(PolicyDryRunRequest{
		Direction: "sideways",
		Bundle: PolicyBundle{
			Key:           "pii",
			Version:       "v1",
			Source:        "user",
			DefaultAction: "allow",
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PolicyDryRun() error = %v, want ErrInvalidInput", err)
	}
}

func TestPolicyDryRunRejectsInvalidBundle(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PolicyDryRun(PolicyDryRunRequest{
		Direction: "input",
		Bundle: PolicyBundle{
			Key:           "pii",
			Version:       "v1",
			Source:        "user",
			DefaultAction: "drop",
		},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PolicyDryRun() error = %v, want ErrInvalidInput", err)
	}
}

func TestControlConfigurationWorkflows(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	if err := service.PutModelMapping(context.Background(), ModelMapping{
		ID:       "chat-openai",
		RouteKey: "chat",
		Provider: "openai",
		Model:    "gpt-4.1-mini",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("PutModelMapping() error = %v", err)
	}
	mappings, err := service.ListModelMappings(context.Background())
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if len(mappings) != 1 || mappings[0].ID != "chat-openai" {
		t.Fatalf("mappings = %+v, want saved chat-openai mapping", mappings)
	}

	if err := service.PutVerdictProvider(context.Background(), VerdictProvider{
		ID:       "sec",
		Name:     "Security model",
		Endpoint: "http://127.0.0.1:9000",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("PutVerdictProvider() error = %v", err)
	}
	providers, err := service.ListVerdictProviders(context.Background())
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Mode != "inline" {
		t.Fatalf("providers = %+v, want one inline provider", providers)
	}

	limit, err := service.PutRouteLimit(context.Background(), RouteLimit{
		RouteKey:              "chat",
		RequestsPerWindow:     10,
		WindowSeconds:         60,
		MaxConcurrentRequests: 2,
		MaxBodyBytes:          4096,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("PutRouteLimit() error = %v", err)
	}
	if limit.RouteKey != "chat" {
		t.Fatalf("route limit = %+v, want chat", limit)
	}
	limits, err := service.ListRouteLimits(context.Background())
	if err != nil {
		t.Fatalf("ListRouteLimits() error = %v", err)
	}
	if len(limits) != 1 || limits[0].RequestsPerWindow != 10 {
		t.Fatalf("route limits = %+v, want saved limit", limits)
	}
}

func TestDefaultServiceUsesMemoryRepository(t *testing.T) {
	service := NewService(ServiceOptions{})

	status, err := service.ConfigStatus(context.Background())
	if err != nil {
		t.Fatalf("ConfigStatus() error = %v", err)
	}
	if len(status.ActivePolicyBundleKeys) != 0 {
		t.Fatalf("ConfigStatus() active keys = %+v, want none", status.ActivePolicyBundleKeys)
	}

	bundles, err := service.ListPolicyBundles(context.Background())
	if err != nil {
		t.Fatalf("ListPolicyBundles() error = %v", err)
	}
	if len(bundles) != 0 {
		t.Fatalf("ListPolicyBundles() = %+v, want none", bundles)
	}

	clients, err := service.ListGatewayClients(context.Background())
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 0 {
		t.Fatalf("ListGatewayClients() = %+v, want none", clients)
	}
}

func TestClientRouteLimitRequiresExistingClient(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PutClientRouteLimit(context.Background(), ClientRouteLimit{
		ClientID:              "missing",
		RouteKey:              "chat",
		RequestsPerWindow:     1,
		WindowSeconds:         60,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
		Enabled:               true,
	})
	if !errors.Is(err, ErrGatewayClientNotFound) {
		t.Fatalf("PutClientRouteLimit() error = %v, want ErrGatewayClientNotFound", err)
	}
}

func TestRouteLimitUseCasesValidateInput(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PutRouteLimit(context.Background(), RouteLimit{
		RouteKey:              "",
		RequestsPerWindow:     1,
		WindowSeconds:         60,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
		Enabled:               true,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutRouteLimit(empty route) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.PutRouteLimit(context.Background(), RouteLimit{
		RouteKey:              "chat",
		RequestsPerWindow:     0,
		WindowSeconds:         60,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
		Enabled:               true,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutRouteLimit(zero requests) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.ListClientRouteLimits(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ListClientRouteLimits(empty client) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.PutClientRouteLimit(context.Background(), ClientRouteLimit{
		ClientID:              "",
		RouteKey:              "chat",
		RequestsPerWindow:     1,
		WindowSeconds:         60,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
		Enabled:               true,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutClientRouteLimit(empty client) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.PutClientRouteLimit(context.Background(), ClientRouteLimit{
		ClientID:              "acme",
		RouteKey:              "chat",
		RequestsPerWindow:     1,
		WindowSeconds:         0,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          1024,
		Enabled:               true,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PutClientRouteLimit(zero window) error = %v, want ErrInvalidInput", err)
	}

	err = service.DeleteClientRouteLimit(context.Background(), "", "chat")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("DeleteClientRouteLimit(empty client) error = %v, want ErrInvalidInput", err)
	}
	err = service.DeleteClientRouteLimit(context.Background(), "acme", "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("DeleteClientRouteLimit(empty route) error = %v, want ErrInvalidInput", err)
	}
}

func TestClientRouteLimitLifecycle(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})
	created, err := service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: "Acme",
	})
	if err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}

	limit, err := service.PutClientRouteLimit(context.Background(), ClientRouteLimit{
		ClientID:              created.Client.ID,
		RouteKey:              "chat",
		RequestsPerWindow:     3,
		WindowSeconds:         60,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          2048,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("PutClientRouteLimit() error = %v", err)
	}
	if limit.ClientID != "acme" || limit.RouteKey != "chat" {
		t.Fatalf("client route limit = %+v, want acme/chat", limit)
	}

	limits, err := service.ListClientRouteLimits(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ListClientRouteLimits() error = %v", err)
	}
	if len(limits) != 1 {
		t.Fatalf("client route limits = %+v, want one item", limits)
	}

	if err := service.DeleteClientRouteLimit(context.Background(), "acme", "chat"); err != nil {
		t.Fatalf("DeleteClientRouteLimit() error = %v", err)
	}
	limits, err = service.ListClientRouteLimits(context.Background(), "acme")
	if err != nil {
		t.Fatalf("ListClientRouteLimits() after delete error = %v", err)
	}
	if len(limits) != 0 {
		t.Fatalf("client route limits after delete = %+v, want none", limits)
	}
}

func TestPatchAndRevokeGatewayClient(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})
	created, err := service.CreateGatewayClient(context.Background(), CreateGatewayClientRequest{
		ID:   "acme",
		Name: "Acme",
	})
	if err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}

	patched, err := service.PatchGatewayClient(context.Background(), GatewayClient{
		ID:     created.Client.ID,
		Name:   "Acme Updated",
		Status: "disabled",
		Notes:  "paused",
	})
	if err != nil {
		t.Fatalf("PatchGatewayClient() error = %v", err)
	}
	if patched.Name != "Acme Updated" || patched.KeyPrefix != created.Client.KeyPrefix {
		t.Fatalf("patched client = %+v, want updated name with preserved key prefix", patched)
	}

	revoked, err := service.RevokeGatewayClient(context.Background(), created.Client.ID)
	if err != nil {
		t.Fatalf("RevokeGatewayClient() error = %v", err)
	}
	if revoked.Status != "revoked" {
		t.Fatalf("revoked status = %q, want revoked", revoked.Status)
	}
}

func TestGatewayClientUseCasesValidateInput(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PatchGatewayClient(context.Background(), GatewayClient{
		ID:     "acme",
		Name:   "",
		Status: "enabled",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PatchGatewayClient(empty name) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.PatchGatewayClient(context.Background(), GatewayClient{
		ID:     "acme",
		Name:   "Acme",
		Status: "paused",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("PatchGatewayClient(invalid status) error = %v, want ErrInvalidInput", err)
	}

	_, err = service.RevokeGatewayClient(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("RevokeGatewayClient(empty id) error = %v, want ErrInvalidInput", err)
	}
}

func TestGatewayClientUseCasesPropagateRepositoryErrors(t *testing.T) {
	service := NewService(ServiceOptions{Repository: NewMemoryRepository()})

	_, err := service.PatchGatewayClient(context.Background(), GatewayClient{
		ID:     "missing",
		Name:   "Missing",
		Status: "enabled",
	})
	if !errors.Is(err, ErrGatewayClientNotFound) {
		t.Fatalf("PatchGatewayClient(missing) error = %v, want ErrGatewayClientNotFound", err)
	}

	_, err = service.RevokeGatewayClient(context.Background(), "missing")
	if !errors.Is(err, ErrGatewayClientNotFound) {
		t.Fatalf("RevokeGatewayClient(missing) error = %v, want ErrGatewayClientNotFound", err)
	}
}

func minimalPolicyBundle(key string) PolicyBundle {
	return PolicyBundle{
		Key:           key,
		Version:       "v1",
		Source:        "user",
		DefaultAction: "allow",
	}
}

type failingNotifier struct {
	err error
}

func (n failingNotifier) NotifyConfigActivated(context.Context, int64) error {
	return n.err
}

type recordingNotifier struct {
	calls int
}

func (n *recordingNotifier) NotifyConfigActivated(context.Context, int64) error {
	n.calls++
	return nil
}

type failingPolicyRepository struct {
	*MemoryRepository
	listPolicyBundlesErr     error
	activatePolicyBundlesErr error
}

func (r *failingPolicyRepository) ListPolicyBundles(ctx context.Context) ([]PolicyBundle, error) {
	if r.listPolicyBundlesErr != nil {
		return nil, r.listPolicyBundlesErr
	}
	return r.MemoryRepository.ListPolicyBundles(ctx)
}

func (r *failingPolicyRepository) ActivatePolicyBundles(ctx context.Context, request PolicyActivationRequest) (PolicyActivationResponse, error) {
	if r.activatePolicyBundlesErr != nil {
		return PolicyActivationResponse{}, r.activatePolicyBundlesErr
	}
	return r.MemoryRepository.ActivatePolicyBundles(ctx, request)
}

type recordingRepository struct {
	*MemoryRepository
	createGatewayClientCalled bool
	createGatewayClientErr    error
}

func (r *recordingRepository) CreateGatewayClient(ctx context.Context, request CreateGatewayClientRequest) (CreateGatewayClientResponse, error) {
	r.createGatewayClientCalled = true
	if r.createGatewayClientErr != nil {
		return CreateGatewayClientResponse{}, r.createGatewayClientErr
	}
	return r.MemoryRepository.CreateGatewayClient(ctx, request)
}
