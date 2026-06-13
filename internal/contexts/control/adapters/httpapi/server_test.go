package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
)

func TestHealthEndpoint(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("Status = %q, want ok", body.Status)
	}
	if body.Version != "test" {
		t.Fatalf("Version = %q, want test", body.Version)
	}
}

func TestHealthEndpointReportsInjectedHealth(t *testing.T) {
	server := NewServer(ServerOptions{
		Version:      "test",
		ConfigHealth: staticConfigHealth{ready: false, reason: "config_subscriber_closed"},
		AuditHealth:  staticAuditHealth{healthy: false, reason: "clickhouse_unhealthy"},
		SpoolStatus: staticSpoolStatus{status: SpoolStatus{
			Enabled:          true,
			Status:           "degraded",
			Reason:           "event_spool_overflow",
			Depth:            3,
			Bytes:            2048,
			MaxBytes:         4096,
			OldestAgeSeconds: 12.5,
			OverflowCount:    2,
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var body struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status           string  `json:"status"`
			Reason           string  `json:"reason"`
			Depth            int     `json:"depth"`
			Bytes            int64   `json:"bytes"`
			MaxBytes         int64   `json:"max_bytes"`
			OldestAgeSeconds float64 `json:"oldest_age_seconds"`
			OverflowCount    uint64  `json:"overflow_count"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Status != "unhealthy" {
		t.Fatalf("body status = %q, want unhealthy", body.Status)
	}
	if body.Checks["config"].Status != "degraded" || body.Checks["config"].Reason != "config_subscriber_closed" {
		t.Fatalf("config check = %+v, want degraded config subscriber reason", body.Checks["config"])
	}
	if body.Checks["audit_sink"].Status != "unhealthy" || body.Checks["audit_sink"].Reason != "clickhouse_unhealthy" {
		t.Fatalf("audit check = %+v, want unhealthy audit sink reason", body.Checks["audit_sink"])
	}
	if body.Checks["event_spool"].Status != "degraded" || body.Checks["event_spool"].Depth != 3 || body.Checks["event_spool"].OverflowCount != 2 {
		t.Fatalf("event_spool check = %+v, want degraded spool stats", body.Checks["event_spool"])
	}
}

func TestTrafficSpoolEndpointReportsLiveStats(t *testing.T) {
	server := NewServer(ServerOptions{
		Version: "test",
		SpoolStatus: staticSpoolStatus{status: SpoolStatus{
			Enabled:          true,
			Status:           "ok",
			Depth:            7,
			Bytes:            1024,
			MaxBytes:         1 << 20,
			OldestAgeSeconds: 3.25,
			OverflowCount:    1,
		}},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/traffic/spool", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body SpoolStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if !body.Enabled || body.Status != "ok" || body.Depth != 7 || body.Bytes != 1024 || body.MaxBytes != 1<<20 {
		t.Fatalf("spool status = %+v, want live stats", body)
	}
	if body.OldestAgeSeconds != 3.25 || body.OverflowCount != 1 {
		t.Fatalf("spool age/overflow = %+v, want age and overflow", body)
	}
}

func TestTrafficSpoolEndpointReturnsUnavailableWithoutProvider(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/traffic/spool", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Code != "spool_status_unavailable" {
		t.Fatalf("Code = %q, want spool_status_unavailable", body.Code)
	}
}

func TestConsoleSummaryEndpointAggregatesControlPlaneState(t *testing.T) {
	store := newFakePolicyStore()
	store.configStatus = configStatusResponse{
		ActivePolicyBundleKeys: []string{"pii", "secrets"},
		PolicySnapshotHash:     "snapshot-hash",
	}
	store.bundles = []policyBundleDTO{
		{Key: "pii", Version: "2026.05", Source: "user", DefaultAction: "allow"},
		{Key: "secrets", Version: "2026.05", Source: "user", DefaultAction: "block"},
		{Key: "audit", Version: "2026.05", Source: "system", DefaultAction: "allow"},
	}
	store.modelMappings = []modelMappingDTO{
		{ID: "openai", RouteKey: "openai", Provider: "openai", Model: "gpt-4.1", Enabled: true},
		{ID: "safe", RouteKey: "safe", Provider: "vertical-security", Model: "guard", Enabled: true},
	}
	store.verdictProviders = []verdictProviderDTO{
		{ID: "guard", Name: "Guard", Endpoint: "http://guard.local", Mode: "enforce", Enabled: true},
	}
	traffic := &fakeTrafficReader{
		response: trafficEventsResponse{
			Summary: trafficEventsSummaryDTO{
				Total:          18,
				Blocked:        3,
				UpstreamErrors: 2,
				Fallbacks:      1,
			},
		},
	}
	server := NewServer(ServerOptions{
		Version:       "test-version",
		Store:         store,
		SpoolStatus:   staticSpoolStatus{status: SpoolStatus{Enabled: true, Status: "ok", Depth: 4, Bytes: 2048, MaxBytes: 1 << 20, OldestAgeSeconds: 1.5, OverflowCount: 2}},
		TrafficReader: traffic,
	})

	rec := get(t, server, "/api/console/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body consoleSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "test-version" {
		t.Fatalf("Version = %q, want test-version", body.Version)
	}
	if !body.Config.Available || body.Config.PolicySnapshotHash != "snapshot-hash" {
		t.Fatalf("config summary = %+v, want available snapshot status", body.Config)
	}
	if body.Policy.ActiveBundleKeyCount != 2 || body.Policy.BundleCount != 3 {
		t.Fatalf("policy summary = %+v, want active=2 bundles=3", body.Policy)
	}
	if body.Routing.ModelMappingCount != 2 || body.Routing.VerdictProviderCount != 1 {
		t.Fatalf("routing summary = %+v, want mappings=2 providers=1", body.Routing)
	}
	if body.Traffic.Total != 18 || body.Traffic.Blocked != 3 || body.Traffic.UpstreamErrors != 2 || body.Traffic.Fallbacks != 1 {
		t.Fatalf("traffic summary = %+v, want reader summary", body.Traffic)
	}
	if !body.Storage.Available || !body.Storage.Enabled || body.Storage.Status != "ok" || body.Storage.Depth != 4 || body.Storage.Bytes != 2048 || body.Storage.MaxBytes != 1<<20 {
		t.Fatalf("storage summary = %+v, want spool state", body.Storage)
	}
	if body.Storage.OldestAgeSeconds != 1.5 || body.Storage.OverflowCount != 2 {
		t.Fatalf("storage age/overflow = %+v, want age=1.5 overflow=2", body.Storage)
	}
	if traffic.filter != (trafficEventFilter{}) {
		t.Fatalf("ListTrafficEvents filter = %+v, want no list-item query", traffic.filter)
	}
	if traffic.summaryFilter != (trafficEventFilter{}) {
		t.Fatalf("summary filter = %+v, want unfiltered traffic summary", traffic.summaryFilter)
	}
}

func TestConsoleSummaryEndpointReturnsZeroValuesForMissingDependencies(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test-version"})

	rec := get(t, server, "/api/console/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body consoleSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Version != "test-version" {
		t.Fatalf("Version = %q, want test-version", body.Version)
	}
	if body.Config.Available || body.Storage.Available {
		t.Fatalf("availability = config:%v storage:%v, want false for nil dependencies", body.Config.Available, body.Storage.Available)
	}
	if body.Policy.BundleCount != 0 || body.Routing.ModelMappingCount != 0 || body.Traffic.Total != 0 || body.Storage.Depth != 0 {
		t.Fatalf("summary = %+v, want zero-value subsystem counts", body)
	}
}

func TestConsoleSummaryEndpointReturnsJSONArraysForEmptyConfig(t *testing.T) {
	store := newFakePolicyStore()
	server := NewServer(ServerOptions{Version: "test-version", Store: store})

	rec := get(t, server, "/api/console/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"active_policy_bundle_keys":[]`) {
		t.Fatalf("body = %s, want active_policy_bundle_keys encoded as []", rec.Body.String())
	}
}

func TestConsoleSummaryEndpointReturnsTrafficSummaryError(t *testing.T) {
	server := NewServer(ServerOptions{
		Version:       "test-version",
		TrafficReader: &fakeTrafficReader{err: errors.New("summary failed")},
	})

	rec := get(t, server, "/api/console/summary")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rec.Code, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "console_summary_failed" || body.Message != "traffic summary query failed" {
		t.Fatalf("error = %+v, want traffic summary failure", body)
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	req := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestMetricsEndpointUsesInjectedHandler(t *testing.T) {
	server := NewServer(ServerOptions{
		Version: "test",
		MetricsHandler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("metrics ok"))
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if rec.Body.String() != "metrics ok" {
		t.Fatalf("body = %q, want metrics ok", rec.Body.String())
	}
}

func TestBearerTokenProtectsMutatingControlEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test", BearerToken: "control-secret"})

	rec := postJSON(t, server, "/api/tools/regex-test", `{"pattern":"test","text":"test"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tools/regex-test", strings.NewReader(`{"pattern":"test","text":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer control-secret")
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestBearerTokenLeavesHealthEndpointOpen(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test", BearerToken: "control-secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegexTestEndpoint(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	req := httptest.NewRequest(http.MethodPost, "/api/tools/regex-test", strings.NewReader(`{"pattern":"[a-z]+@[a-z]+\\.com","text":"alice@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Matches []struct {
			Start int    `json:"start"`
			End   int    `json:"end"`
			Text  string `json:"text"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(body.Matches) != 1 {
		t.Fatalf("len(Matches) = %d, want 1", len(body.Matches))
	}
	if body.Matches[0].Text != "alice@example.com" {
		t.Fatalf("Text = %q, want alice@example.com", body.Matches[0].Text)
	}
}

func TestPolicyBundleLifecycleEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})
	bundleJSON := `{
		"key":"pii",
		"version":"2026.05",
		"source":"user",
		"default_action":"allow",
		"detectors":[{"key":"email","kind":"regex","pattern":"[a-z]+@[a-z]+\\.com","categories":["pii.email"]}],
		"rules":[{"key":"block-email","enabled":true,"severity":"high","action":"block","detector_keys":["email"],"scope":{"direction":"input"}}]
	}`

	rec := postJSON(t, server, "/api/policy-bundles/validate", bundleJSON)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var validation struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &validation); err != nil {
		t.Fatalf("json.Unmarshal(validation) error = %v", err)
	}
	if !validation.Valid {
		t.Fatal("Valid = false, want true")
	}

	rec = postJSON(t, server, "/api/policy-bundles", bundleJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}

	rec = get(t, server, "/api/policy-bundles")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			Key string `json:"key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Key != "pii" {
		t.Fatalf("Items = %+v, want one pii bundle", list.Items)
	}

	rec = postJSON(t, server, "/api/policy-bundles/activate", `{"keys":["pii"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var activation struct {
		ActiveKeys []string `json:"active_keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &activation); err != nil {
		t.Fatalf("json.Unmarshal(activation) error = %v", err)
	}
	if len(activation.ActiveKeys) != 1 || activation.ActiveKeys[0] != "pii" {
		t.Fatalf("ActiveKeys = %+v, want [pii]", activation.ActiveKeys)
	}

	rec = get(t, server, "/api/config/active")
	if rec.Code != http.StatusOK {
		t.Fatalf("active config status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var active struct {
		ActivePolicyBundleKeys []string `json:"active_policy_bundle_keys"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &active); err != nil {
		t.Fatalf("json.Unmarshal(active) error = %v", err)
	}
	if len(active.ActivePolicyBundleKeys) != 1 || active.ActivePolicyBundleKeys[0] != "pii" {
		t.Fatalf("ActivePolicyBundleKeys = %+v, want [pii]", active.ActivePolicyBundleKeys)
	}
}

func TestCanonicalDomainAPIAliases(t *testing.T) {
	store := newFakePolicyStore()
	store.activatePolicyBundlesErr = errors.New("keys are required")
	server := NewServer(ServerOptions{
		Version: "test",
		Store:   store,
		SpoolStatus: staticSpoolStatus{status: SpoolStatus{
			Enabled: true,
			Status:  "ok",
			Depth:   4,
		}},
	})
	bundleJSON := `{
		"key":"pii",
		"version":"2026.05",
		"source":"user",
		"default_action":"allow",
		"detectors":[{"key":"email","kind":"regex","pattern":"[a-z]+@[a-z]+\\.com","categories":["pii.email"]}],
		"rules":[{"key":"block-email","enabled":true,"severity":"high","action":"block","detector_keys":["email"],"scope":{"direction":"input"}}]
	}`

	rec := get(t, server, "/api/policies/bundles")
	if rec.Code != http.StatusOK {
		t.Fatalf("list policy bundles alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = postJSON(t, server, "/api/policies/bundles/validate", bundleJSON)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate policy bundle alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = postJSON(t, server, "/api/policies/bundles", bundleJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create policy bundle alias status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if store.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", store.createCalls)
	}
	if len(store.bundles) != 1 || store.bundles[0].Key != "pii" {
		t.Fatalf("store bundles = %+v, want one pii bundle", store.bundles)
	}

	rec = postJSON(t, server, "/api/policies/bundles/activate", `{"keys":[]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("activate policy bundles alias status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var activationError errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &activationError); err != nil {
		t.Fatalf("json.Unmarshal(activationError) error = %v", err)
	}
	if activationError.Code != "activate_policy_bundles_failed" {
		t.Fatalf("activation error code = %q, want activate_policy_bundles_failed", activationError.Code)
	}

	rec = putJSON(t, server, "/api/routing/model-mappings/default", `{"route_key":"chat","provider":"openai","model":"gpt-test","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put model mapping alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var mapping modelMappingDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &mapping); err != nil {
		t.Fatalf("json.Unmarshal(mapping) error = %v", err)
	}
	if mapping.ID != "default" {
		t.Fatalf("mapping ID = %q, want default", mapping.ID)
	}

	rec = get(t, server, "/api/routing/model-mappings")
	if rec.Code != http.StatusOK {
		t.Fatalf("list model mappings alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = putJSON(t, server, "/api/providers/verdict/sec-model", `{"name":"Security Model","endpoint":"http://verdict.test/evaluate","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put verdict provider alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var provider verdictProviderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &provider); err != nil {
		t.Fatalf("json.Unmarshal(provider) error = %v", err)
	}
	if provider.ID != "sec-model" {
		t.Fatalf("provider ID = %q, want sec-model", provider.ID)
	}

	rec = get(t, server, "/api/providers/verdict")
	if rec.Code != http.StatusOK {
		t.Fatalf("list verdict providers alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = get(t, server, "/api/storage/event-spool")
	if rec.Code != http.StatusOK {
		t.Fatalf("event spool alias status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGatewayClientLifecycleEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/gateway-clients", `{"name":"Acme Beta","notes":"Pilot tenant"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Client struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			KeyPrefix string `json:"key_prefix"`
			Notes     string `json:"notes"`
		} `json:"client"`
		Key string `json:"key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(created) error = %v", err)
	}
	if created.Client.ID == "" || created.Client.KeyPrefix == "" || created.Key == "" {
		t.Fatalf("created = %+v, want client id, key prefix, and one-time key", created)
	}
	if created.Client.Name != "Acme Beta" || created.Client.Status != "enabled" {
		t.Fatalf("created client = %+v, want enabled Acme Beta", created.Client)
	}
	if created.Client.Notes != "Pilot tenant" {
		t.Fatalf("created client notes = %q, want Pilot tenant", created.Client.Notes)
	}

	rec = get(t, server, "/api/gateway-clients")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), created.Key) {
		t.Fatalf("list response leaked one-time key %q: %s", created.Key, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			KeyPrefix string `json:"key_prefix"`
			Notes     string `json:"notes"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.Client.ID || list.Items[0].KeyPrefix == "" {
		t.Fatalf("items = %+v, want created client without raw key", list.Items)
	}
	if list.Items[0].Notes != "Pilot tenant" {
		t.Fatalf("listed client notes = %q, want Pilot tenant", list.Items[0].Notes)
	}

	rec = patchJSON(t, server, "/api/gateway-clients/"+created.Client.ID, `{"name":"Acme Beta Updated","status":"disabled","notes":"Requires monthly review"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var patched struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		KeyPrefix string `json:"key_prefix"`
		Notes     string `json:"notes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &patched); err != nil {
		t.Fatalf("json.Unmarshal(patched) error = %v", err)
	}
	if patched.ID != created.Client.ID || patched.Name != "Acme Beta Updated" || patched.Status != "disabled" || patched.KeyPrefix == "" {
		t.Fatalf("patched = %+v, want disabled updated client with key prefix", patched)
	}
	if patched.Notes != "Requires monthly review" {
		t.Fatalf("patched notes = %q, want Requires monthly review", patched.Notes)
	}

	rec = postJSON(t, server, "/api/gateway-clients/"+created.Client.ID+"/revoke", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var revoked struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &revoked); err != nil {
		t.Fatalf("json.Unmarshal(revoked) error = %v", err)
	}
	if revoked.ID != created.Client.ID || revoked.Status != "revoked" {
		t.Fatalf("revoked = %+v, want revoked created client", revoked)
	}
}

func TestGatewayClientMissingPathErrors(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	tests := []struct {
		name string
		run  func() *httptest.ResponseRecorder
	}{
		{
			name: "patch missing client",
			run: func() *httptest.ResponseRecorder {
				return patchJSON(t, server, "/api/gateway-clients/missing", `{"name":"Missing","status":"enabled"}`)
			},
		},
		{
			name: "revoke missing client",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/gateway-clients/missing/revoke", `{}`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.run()
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if body.Code != "gateway_client_not_found" || body.Message != "gateway client not found" {
				t.Fatalf("error = %+v, want gateway client not found", body)
			}
		})
	}
}

func TestGatewayRouteLimitEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})
	body := `{"requests_per_window":10,"window_seconds":60,"max_concurrent_requests":2,"max_body_bytes":4096,"enabled":true}`

	rec := putJSON(t, server, "/api/gateway-limits/routes/chat", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var saved struct {
		RouteKey              string `json:"route_key"`
		RequestsPerWindow     int    `json:"requests_per_window"`
		WindowSeconds         int    `json:"window_seconds"`
		MaxConcurrentRequests int    `json:"max_concurrent_requests"`
		MaxBodyBytes          int64  `json:"max_body_bytes"`
		Enabled               bool   `json:"enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("json.Unmarshal(saved) error = %v", err)
	}
	if saved.RouteKey != "chat" || saved.RequestsPerWindow != 10 || !saved.Enabled {
		t.Fatalf("saved = %+v, want enabled chat route limit", saved)
	}

	rec = get(t, server, "/api/gateway-limits/routes")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			RouteKey string `json:"route_key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].RouteKey != "chat" {
		t.Fatalf("items = %+v, want chat route limit", list.Items)
	}
}

func TestGatewayClientRouteOverrideEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})
	created := postJSON(t, server, "/api/gateway-clients", `{"name":"Acme Beta"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", created.Code, created.Body.String())
	}
	var clientResponse struct {
		Client struct {
			ID string `json:"id"`
		} `json:"client"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &clientResponse); err != nil {
		t.Fatalf("json.Unmarshal(clientResponse) error = %v", err)
	}

	body := `{"requests_per_window":3,"window_seconds":30,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`
	path := "/api/gateway-limits/clients/" + clientResponse.Client.ID + "/routes/chat"
	rec := putJSON(t, server, path, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var saved struct {
		ClientID          string `json:"client_id"`
		RouteKey          string `json:"route_key"`
		RequestsPerWindow int    `json:"requests_per_window"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("json.Unmarshal(saved) error = %v", err)
	}
	if saved.ClientID != clientResponse.Client.ID || saved.RouteKey != "chat" || saved.RequestsPerWindow != 3 {
		t.Fatalf("saved = %+v, want client chat override", saved)
	}

	rec = get(t, server, "/api/gateway-limits/clients/"+clientResponse.Client.ID+"/routes")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ClientID string `json:"client_id"`
			RouteKey string `json:"route_key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ClientID != clientResponse.Client.ID || list.Items[0].RouteKey != "chat" {
		t.Fatalf("items = %+v, want client chat override", list.Items)
	}

	rec = deleteJSON(t, server, path)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	rec = get(t, server, "/api/gateway-limits/clients/"+clientResponse.Client.ID+"/routes")
	if rec.Code != http.StatusOK {
		t.Fatalf("list after delete status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	list.Items = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list after delete) error = %v", err)
	}
	if len(list.Items) != 0 {
		t.Fatalf("items after delete = %+v, want empty", list.Items)
	}
}

func TestGatewayLimitValidationRejectsInvalidValues(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	tests := []struct {
		name       string
		run        func() *httptest.ResponseRecorder
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "empty client name",
			run:        func() *httptest.ResponseRecorder { return postJSON(t, server, "/api/gateway-clients", `{"name":""}`) },
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_gateway_client",
			wantMsg:    "client name is required",
		},
		{
			name: "unknown client status",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/gateway-clients", `{"name":"Acme","status":"paused"}`)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_gateway_client",
			wantMsg:    "client status must be enabled, disabled, or revoked",
		},
		{
			name: "empty route key",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/gateway-limits/routes/%20", `{"requests_per_window":1,"window_seconds":60,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_route_limit",
			wantMsg:    "route key is required",
		},
		{
			name: "invalid route limit values",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/gateway-limits/routes/chat", `{"requests_per_window":0,"window_seconds":60,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_route_limit",
			wantMsg:    "limit values must be greater than zero",
		},
		{
			name: "missing client id",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/gateway-limits/clients/%20/routes/chat", `{"requests_per_window":1,"window_seconds":60,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_client_route_limit",
			wantMsg:    "client_id is required",
		},
		{
			name: "invalid override values",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/gateway-limits/clients/client-a/routes/chat", `{"requests_per_window":1,"window_seconds":0,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`)
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_client_route_limit",
			wantMsg:    "limit values must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.run()
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if body.Code != tt.wantCode || body.Message != tt.wantMsg {
				t.Fatalf("error = %+v, want code %q message %q", body, tt.wantCode, tt.wantMsg)
			}
		})
	}
}

func TestGatewayStoreBackedEndpointErrors(t *testing.T) {
	limitJSON := `{"requests_per_window":1,"window_seconds":60,"max_concurrent_requests":1,"max_body_bytes":1024,"enabled":true}`
	tests := []struct {
		name       string
		configure  func(*fakePolicyStore)
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "list gateway clients",
			configure:  func(store *fakePolicyStore) { store.listGatewayClientsErr = errors.New("list failed") },
			method:     http.MethodGet,
			path:       "/api/gateway-clients",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "list_gateway_clients_failed",
		},
		{
			name:       "create gateway client duplicate",
			configure:  func(store *fakePolicyStore) { store.createGatewayClientErr = appcontrol.ErrGatewayClientAlreadyExists },
			method:     http.MethodPost,
			path:       "/api/gateway-clients",
			body:       `{"id":"client-a","name":"Acme"}`,
			wantStatus: http.StatusConflict,
			wantCode:   "gateway_client_exists",
		},
		{
			name:       "create gateway client repository failure",
			configure:  func(store *fakePolicyStore) { store.createGatewayClientErr = errors.New("create failed") },
			method:     http.MethodPost,
			path:       "/api/gateway-clients",
			body:       `{"id":"client-a","name":"Acme"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "create_gateway_client_failed",
		},
		{
			name:       "patch gateway client repository failure",
			configure:  func(store *fakePolicyStore) { store.patchGatewayClientErr = errors.New("patch failed") },
			method:     http.MethodPatch,
			path:       "/api/gateway-clients/client-a",
			body:       `{"name":"Acme","status":"enabled"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "patch_gateway_client_failed",
		},
		{
			name:       "revoke gateway client repository failure",
			configure:  func(store *fakePolicyStore) { store.revokeGatewayClientErr = errors.New("revoke failed") },
			method:     http.MethodPost,
			path:       "/api/gateway-clients/client-a/revoke",
			body:       `{}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "revoke_gateway_client_failed",
		},
		{
			name:       "list route limits",
			configure:  func(store *fakePolicyStore) { store.listRouteLimitsErr = errors.New("list failed") },
			method:     http.MethodGet,
			path:       "/api/gateway-limits/routes",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "list_route_limits_failed",
		},
		{
			name:       "put route limit repository failure",
			configure:  func(store *fakePolicyStore) { store.putRouteLimitErr = errors.New("put failed") },
			method:     http.MethodPut,
			path:       "/api/gateway-limits/routes/chat",
			body:       limitJSON,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "put_route_limit_failed",
		},
		{
			name:       "list client route limits missing client",
			configure:  func(store *fakePolicyStore) { store.listClientRouteLimitsErr = appcontrol.ErrGatewayClientNotFound },
			method:     http.MethodGet,
			path:       "/api/gateway-limits/clients/client-a/routes",
			wantStatus: http.StatusNotFound,
			wantCode:   "gateway_client_not_found",
		},
		{
			name:       "put client route limit missing client",
			configure:  func(store *fakePolicyStore) { store.putClientRouteLimitErr = appcontrol.ErrGatewayClientNotFound },
			method:     http.MethodPut,
			path:       "/api/gateway-limits/clients/client-a/routes/chat",
			body:       limitJSON,
			wantStatus: http.StatusNotFound,
			wantCode:   "gateway_client_not_found",
		},
		{
			name:       "delete client route limit repository failure",
			configure:  func(store *fakePolicyStore) { store.deleteClientRouteLimitErr = errors.New("delete failed") },
			method:     http.MethodDelete,
			path:       "/api/gateway-limits/clients/client-a/routes/chat",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "delete_client_route_limit_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakePolicyStore()
			tt.configure(store)
			server := NewServer(ServerOptions{Version: "test", Store: store})

			var rec *httptest.ResponseRecorder
			switch tt.method {
			case http.MethodGet:
				rec = get(t, server, tt.path)
			case http.MethodPost:
				rec = postJSON(t, server, tt.path, tt.body)
			case http.MethodPut:
				rec = putJSON(t, server, tt.path, tt.body)
			case http.MethodPatch:
				rec = patchJSON(t, server, tt.path, tt.body)
			case http.MethodDelete:
				rec = deleteJSON(t, server, tt.path)
			default:
				t.Fatalf("unsupported method %s", tt.method)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("Code = %q, want %s; body=%s", body.Code, tt.wantCode, rec.Body.String())
			}
		})
	}
}

func TestPolicyDryRunEndpoint(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/tools/policy-dry-run", `{
		"route_key":"chat",
		"model":"gpt-test",
		"direction":"input",
		"text":"alice@example.com",
		"bundle":{
			"key":"pii",
			"version":"2026.05",
			"source":"user",
			"default_action":"allow",
			"detectors":[{"key":"email","kind":"regex","pattern":"[a-z]+@[a-z]+\\.com"}],
			"rules":[{"key":"block-email","enabled":true,"severity":"high","action":"block","detector_keys":["email"]}]
		}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Decision struct {
			Action   string `json:"action"`
			RuleHits []struct {
				RuleKey string `json:"rule_key"`
			} `json:"rule_hits"`
		} `json:"decision"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Decision.Action != "block" {
		t.Fatalf("Action = %q, want block", body.Decision.Action)
	}
	if len(body.Decision.RuleHits) != 1 || body.Decision.RuleHits[0].RuleKey != "block-email" {
		t.Fatalf("RuleHits = %+v, want block-email", body.Decision.RuleHits)
	}
}

func TestPolicyDryRunRejectsInvalidDirectionBeforeCompilingBundle(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/tools/policy-dry-run", `{
		"direction":"sideways",
		"bundle":{
			"key":"bad",
			"version":"2026.05",
			"source":"not-a-source",
			"default_action":"allow"
		}
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Code != "invalid_direction" {
		t.Fatalf("Code = %q, want invalid_direction", body.Code)
	}
}

func TestPolicyDryRunRejectsInvalidBundle(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/tools/policy-dry-run", `{
		"direction":"input",
		"bundle":{
			"key":"bad",
			"version":"2026.05",
			"source":"user",
			"default_action":"drop"
		}
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Code != "invalid_policy_bundle" {
		t.Fatalf("Code = %q, want invalid_policy_bundle", body.Code)
	}
}

func TestValidatePolicyBundleReportsInvalidInputAsValidFalse(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/policy-bundles/validate", `{
		"key":"bad",
		"version":"2026.05",
		"source":"user",
		"default_action":"allow",
		"detectors":[{"key":"detector","kind":"unknown"}]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body policyValidationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Valid {
		t.Fatal("Valid = true, want false")
	}
	if body.Error == "" {
		t.Fatal("Error is empty, want validation message")
	}
}

func TestPolicyBundleValidationRejectsUnknownEnums(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/policy-bundles", `{
		"key":"bad",
		"version":"2026.05",
		"source":"user",
		"default_action":"allow",
		"detectors":[{"key":"email","kind":"regex","pattern":"[a-z]+@[a-z]+\\.com"}],
		"rules":[{"key":"bad-direction","enabled":true,"severity":"high","action":"block","detector_keys":["email"],"scope":{"direction":"inpt"}}]
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestPolicyBundleDTOValidateRejectsInvalidEnums(t *testing.T) {
	tests := []struct {
		name   string
		bundle policyBundleDTO
	}{
		{
			name: "source",
			bundle: policyBundleDTO{
				Source:        "remote",
				DefaultAction: "allow",
			},
		},
		{
			name: "severity",
			bundle: policyBundleDTO{
				Source:        "user",
				DefaultAction: "allow",
				Detectors:     []detectorDTO{{Key: "email", Kind: "email"}},
				Rules:         []ruleDTO{{Key: "rule", Severity: "urgent", Action: "block"}},
			},
		},
		{
			name: "rule action",
			bundle: policyBundleDTO{
				Source:        "user",
				DefaultAction: "allow",
				Detectors:     []detectorDTO{{Key: "email", Kind: "email"}},
				Rules:         []ruleDTO{{Key: "rule", Severity: "high", Action: "drop"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := policyBundleToApp(tt.bundle).Validate(); err == nil {
				t.Fatal("validate() error = nil, want enum validation error")
			}
		})
	}
}

func TestModelMappingAndVerdictProviderEndpoints(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := get(t, server, "/api/model-mappings")
	if rec.Code != http.StatusOK {
		t.Fatalf("model mappings status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = putJSON(t, server, "/api/model-mappings/default", `{"id":"default","route_key":"chat","provider":"openai","model":"gpt-test","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put model mapping status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = get(t, server, "/api/verdict-providers")
	if rec.Code != http.StatusOK {
		t.Fatalf("verdict providers status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = putJSON(t, server, "/api/verdict-providers/sec-model", `{"id":"sec-model","name":"Vertical Security Model","endpoint":"http://verdict.test/evaluate","mode":"inline","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put verdict provider status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestStoreBackedModelMappingEndpoints(t *testing.T) {
	store := newFakePolicyStore()
	store.modelMappings = []modelMappingDTO{{
		ID:       "existing",
		RouteKey: "chat",
		Provider: "openai",
		Model:    "gpt-4.1-mini",
		Enabled:  true,
	}}
	server := NewServer(ServerOptions{Version: "test", Store: store})

	rec := get(t, server, "/api/model-mappings")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list modelMappingListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != "existing" {
		t.Fatalf("Items = %+v, want existing mapping", list.Items)
	}

	rec = putJSON(t, server, "/api/model-mappings/default", `{"route_key":"chat","provider":"anthropic","model":"claude-test","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var mapping modelMappingDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &mapping); err != nil {
		t.Fatalf("json.Unmarshal(mapping) error = %v", err)
	}
	if mapping.ID != "default" {
		t.Fatalf("ID = %q, want default", mapping.ID)
	}
	if store.putModelMappingCalls != 1 || store.modelMappings[len(store.modelMappings)-1].Provider != "anthropic" {
		t.Fatalf("store mappings = %+v calls=%d, want saved anthropic mapping", store.modelMappings, store.putModelMappingCalls)
	}
}

func TestStoreBackedVerdictProviderEndpoints(t *testing.T) {
	store := newFakePolicyStore()
	store.verdictProviders = []verdictProviderDTO{{
		ID:       "existing",
		Name:     "Existing Model",
		Endpoint: "http://verdict.test/evaluate",
		Mode:     "inline",
		Enabled:  true,
	}}
	server := NewServer(ServerOptions{Version: "test", Store: store})

	rec := get(t, server, "/api/verdict-providers")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list verdictProviderListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != "existing" {
		t.Fatalf("Items = %+v, want existing provider", list.Items)
	}

	rec = putJSON(t, server, "/api/verdict-providers/sec-model", `{"name":"Security Model","endpoint":"http://verdict.test/evaluate","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var provider verdictProviderDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &provider); err != nil {
		t.Fatalf("json.Unmarshal(provider) error = %v", err)
	}
	if provider.ID != "sec-model" || provider.Mode != "inline" {
		t.Fatalf("provider = %+v, want path id and default inline mode", provider)
	}
	if store.putVerdictProviderCalls != 1 || store.verdictProviders[len(store.verdictProviders)-1].ID != "sec-model" {
		t.Fatalf("store providers = %+v calls=%d, want saved sec-model provider", store.verdictProviders, store.putVerdictProviderCalls)
	}
}

func TestStoreBackedPolicyEndpointErrors(t *testing.T) {
	tests := []struct {
		name       string
		configure  func(*fakePolicyStore)
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "list policy bundles",
			configure:  func(store *fakePolicyStore) { store.listPolicyBundlesErr = errors.New("list failed") },
			method:     http.MethodGet,
			path:       "/api/policy-bundles",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "list_policy_bundles_failed",
		},
		{
			name:       "create policy bundle",
			configure:  func(store *fakePolicyStore) { store.createPolicyBundleErr = errors.New("create failed") },
			method:     http.MethodPost,
			path:       "/api/policy-bundles",
			body:       `{"key":"pii","version":"2026.05","source":"user","default_action":"allow"}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "create_policy_bundle_failed",
		},
		{
			name:       "activate policy bundles",
			configure:  func(store *fakePolicyStore) { store.activatePolicyBundlesErr = errors.New("activate failed") },
			method:     http.MethodPost,
			path:       "/api/policy-bundles/activate",
			body:       `{"keys":["pii"]}`,
			wantStatus: http.StatusBadRequest,
			wantCode:   "activate_policy_bundles_failed",
		},
		{
			name:       "config status",
			configure:  func(store *fakePolicyStore) { store.configStatusErr = errors.New("config failed") },
			method:     http.MethodGet,
			path:       "/api/config/active",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "config_status_failed",
		},
		{
			name:       "list model mappings",
			configure:  func(store *fakePolicyStore) { store.listModelMappingsErr = errors.New("list failed") },
			method:     http.MethodGet,
			path:       "/api/model-mappings",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "list_model_mappings_failed",
		},
		{
			name:       "put model mapping",
			configure:  func(store *fakePolicyStore) { store.putModelMappingErr = errors.New("put failed") },
			method:     http.MethodPut,
			path:       "/api/model-mappings/default",
			body:       `{"provider":"openai","model":"gpt-test","enabled":true}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "put_model_mapping_failed",
		},
		{
			name:       "list verdict providers",
			configure:  func(store *fakePolicyStore) { store.listVerdictProvidersErr = errors.New("list failed") },
			method:     http.MethodGet,
			path:       "/api/verdict-providers",
			wantStatus: http.StatusInternalServerError,
			wantCode:   "list_verdict_providers_failed",
		},
		{
			name:       "put verdict provider",
			configure:  func(store *fakePolicyStore) { store.putVerdictProviderErr = errors.New("put failed") },
			method:     http.MethodPut,
			path:       "/api/verdict-providers/sec-model",
			body:       `{"name":"Security Model","endpoint":"http://verdict.test/evaluate","enabled":true}`,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "put_verdict_provider_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakePolicyStore()
			tt.configure(store)
			server := NewServer(ServerOptions{Version: "test", Store: store})

			var rec *httptest.ResponseRecorder
			switch tt.method {
			case http.MethodGet:
				rec = get(t, server, tt.path)
			case http.MethodPost:
				rec = postJSON(t, server, tt.path, tt.body)
			case http.MethodPut:
				rec = putJSON(t, server, tt.path, tt.body)
			default:
				t.Fatalf("unsupported method %s", tt.method)
			}
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if body.Code != tt.wantCode {
				t.Fatalf("Code = %q, want %s", body.Code, tt.wantCode)
			}
		})
	}
}

func TestControlEndpointsRejectMalformedJSON(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	tests := []struct {
		name string
		run  func() *httptest.ResponseRecorder
	}{
		{
			name: "create policy bundle",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/policy-bundles", `{`)
			},
		},
		{
			name: "validate policy bundle",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/policy-bundles/validate", `{`)
			},
		},
		{
			name: "activate policy bundle",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/policy-bundles/activate", `{`)
			},
		},
		{
			name: "put model mapping",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/model-mappings/default", `{`)
			},
		},
		{
			name: "put verdict provider",
			run: func() *httptest.ResponseRecorder {
				return putJSON(t, server, "/api/verdict-providers/sec-model", `{`)
			},
		},
		{
			name: "regex test",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/tools/regex-test", `{`)
			},
		},
		{
			name: "policy dry run",
			run: func() *httptest.ResponseRecorder {
				return postJSON(t, server, "/api/tools/policy-dry-run", `{`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.run()
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if body.Code != "invalid_json" {
				t.Fatalf("Code = %q, want invalid_json", body.Code)
			}
		})
	}
}

func TestRegexTestRejectsInvalidPattern(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := postJSON(t, server, "/api/tools/regex-test", `{"pattern":"[","text":"alice@example.com"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.Code != "invalid_regex" {
		t.Fatalf("Code = %q, want invalid_regex", body.Code)
	}
}

func TestPolicyBundleLifecycleUsesInjectedStore(t *testing.T) {
	store := newFakePolicyStore()
	server := NewServer(ServerOptions{Version: "test", Store: store})
	bundleJSON := `{
		"key":"pii",
		"version":"2026.05",
		"source":"user",
		"default_action":"allow"
	}`

	rec := postJSON(t, server, "/api/policy-bundles", bundleJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if store.createCalls != 1 {
		t.Fatalf("createCalls = %d, want 1", store.createCalls)
	}

	rec = get(t, server, "/api/policy-bundles")
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var list policyBundleListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("json.Unmarshal(list) error = %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].Key != "pii" {
		t.Fatalf("Items = %+v, want one pii bundle", list.Items)
	}
}

func TestActivatePolicyBundlesPublishesNotification(t *testing.T) {
	store := newFakePolicyStore()
	store.bundles = []policyBundleDTO{{
		Key:           "pii",
		Version:       "2026.05",
		Source:        "user",
		DefaultAction: "allow",
	}}
	notifier := &fakeActivationNotifier{}
	server := NewServer(ServerOptions{Version: "test", Store: store, Notifier: notifier})

	rec := postJSON(t, server, "/api/policy-bundles/activate", `{"keys":["pii"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if store.activateCalls != 1 {
		t.Fatalf("activateCalls = %d, want 1", store.activateCalls)
	}
	if notifier.calls != 1 || notifier.revisionNumber != 7 {
		t.Fatalf("notifier calls = %d revision = %d, want one call for revision 7", notifier.calls, notifier.revisionNumber)
	}
}

func TestActivatePolicyBundlesPropagatesReasonAndReturnsRevisionNumber(t *testing.T) {
	store := newFakePolicyStore()
	store.bundles = []policyBundleDTO{{
		Key:           "pii",
		Version:       "2026.05",
		Source:        "user",
		DefaultAction: "allow",
	}}
	server := NewServer(ServerOptions{Version: "test", Store: store})

	rec := postJSON(t, server, "/api/policy-bundles/activate", `{"keys":["pii"],"reason":"initial rollout"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if store.activationReason != "initial rollout" {
		t.Fatalf("activationReason = %q, want initial rollout", store.activationReason)
	}
	var body struct {
		RevisionNumber int64 `json:"revision_number"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body.RevisionNumber != 7 {
		t.Fatalf("RevisionNumber = %d, want 7; body=%s", body.RevisionNumber, rec.Body.String())
	}
}

func TestActivatePolicyBundlesReturnsOKWhenNotificationFails(t *testing.T) {
	store := newFakePolicyStore()
	store.bundles = []policyBundleDTO{{
		Key:           "pii",
		Version:       "2026.05",
		Source:        "user",
		DefaultAction: "allow",
	}}
	notifier := &fakeActivationNotifier{err: errors.New("notify failed")}
	server := NewServer(ServerOptions{Version: "test", Store: store, Notifier: notifier})

	rec := postJSON(t, server, "/api/policy-bundles/activate", `{"keys":["pii"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body policyActivationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls = %d, want 1", notifier.calls)
	}
	if body.NotificationError != "notify config activation failed" {
		t.Fatalf("NotificationError = %q, want notify config activation failed", body.NotificationError)
	}
}

func TestPutEndpointsRejectIDMismatch(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := putJSON(t, server, "/api/model-mappings/path-id", `{"id":"body-id","route_key":"chat","provider":"openai","model":"gpt-test","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("model mapping status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = putJSON(t, server, "/api/verdict-providers/path-id", `{"id":"body-id","name":"Vertical Security Model","endpoint":"http://verdict.test/evaluate","mode":"inline","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verdict provider status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func TestVerdictProviderRejectsUnsupportedMode(t *testing.T) {
	server := NewServer(ServerOptions{Version: "test"})

	rec := putJSON(t, server, "/api/verdict-providers/sec-model", `{"id":"sec-model","name":"Vertical Security Model","endpoint":"http://verdict.test/evaluate","mode":"async_shadow","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("verdict provider status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func postJSON(t *testing.T, handler http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func putJSON(t *testing.T, handler http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func patchJSON(t *testing.T, handler http.Handler, path string, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func deleteJSON(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type staticAuditHealth struct {
	healthy bool
	reason  string
}

type staticConfigHealth struct {
	ready  bool
	reason string
}

type staticSpoolStatus struct {
	status SpoolStatus
}

func (h staticConfigHealth) ConfigReady() bool {
	return h.ready
}

func (h staticConfigHealth) Reason() string {
	return h.reason
}

func (h staticAuditHealth) Healthy() bool {
	return h.healthy
}

func (h staticAuditHealth) Reason() string {
	return h.reason
}

func (s staticSpoolStatus) SpoolStatus() SpoolStatus {
	return s.status
}

type fakePolicyStore struct {
	configStatus              configStatusResponse
	bundles                   []policyBundleDTO
	modelMappings             []modelMappingDTO
	verdictProviders          []verdictProviderDTO
	gatewayClients            []gatewayClientDTO
	routeLimits               []routeLimitDTO
	clientRouteLimits         []clientRouteLimitDTO
	createCalls               int
	activateCalls             int
	putModelMappingCalls      int
	putVerdictProviderCalls   int
	activationReason          string
	configStatusErr           error
	listPolicyBundlesErr      error
	createPolicyBundleErr     error
	activatePolicyBundlesErr  error
	listModelMappingsErr      error
	putModelMappingErr        error
	listVerdictProvidersErr   error
	putVerdictProviderErr     error
	listGatewayClientsErr     error
	createGatewayClientErr    error
	patchGatewayClientErr     error
	revokeGatewayClientErr    error
	listRouteLimitsErr        error
	putRouteLimitErr          error
	listClientRouteLimitsErr  error
	putClientRouteLimitErr    error
	deleteClientRouteLimitErr error
}

func newFakePolicyStore() *fakePolicyStore {
	return &fakePolicyStore{}
}

func (s *fakePolicyStore) ConfigStatus(ctx context.Context) (appcontrol.ConfigStatus, error) {
	if s.configStatusErr != nil {
		return appcontrol.ConfigStatus{}, s.configStatusErr
	}
	return appcontrol.ConfigStatus{
		ActivePolicyBundleKeys: append([]string(nil), s.configStatus.ActivePolicyBundleKeys...),
		PolicySnapshotHash:     s.configStatus.PolicySnapshotHash,
	}, nil
}

func (s *fakePolicyStore) ListPolicyBundles(ctx context.Context) ([]appcontrol.PolicyBundle, error) {
	if s.listPolicyBundlesErr != nil {
		return nil, s.listPolicyBundlesErr
	}
	return policyBundlesToApp(s.bundles), nil
}

func (s *fakePolicyStore) CreatePolicyBundle(ctx context.Context, bundle appcontrol.PolicyBundle) error {
	if s.createPolicyBundleErr != nil {
		return s.createPolicyBundleErr
	}
	s.createCalls++
	s.bundles = append(s.bundles, policyBundleFromApp(bundle))
	return nil
}

func (s *fakePolicyStore) ActivatePolicyBundles(ctx context.Context, request appcontrol.PolicyActivationRequest) (appcontrol.PolicyActivationResponse, error) {
	if s.activatePolicyBundlesErr != nil {
		return appcontrol.PolicyActivationResponse{}, s.activatePolicyBundlesErr
	}
	s.activateCalls++
	s.activationReason = request.Reason
	return appcontrol.PolicyActivationResponse{ActiveKeys: append([]string(nil), request.Keys...), Hash: "hash", RevisionNumber: 7}, nil
}

func (s *fakePolicyStore) ListModelMappings(ctx context.Context) ([]appcontrol.ModelMapping, error) {
	if s.listModelMappingsErr != nil {
		return nil, s.listModelMappingsErr
	}
	items := make([]appcontrol.ModelMapping, 0, len(s.modelMappings))
	for _, mapping := range s.modelMappings {
		items = append(items, modelMappingToApp(mapping))
	}
	return items, nil
}

func (s *fakePolicyStore) PutModelMapping(ctx context.Context, mapping appcontrol.ModelMapping) error {
	if s.putModelMappingErr != nil {
		return s.putModelMappingErr
	}
	s.putModelMappingCalls++
	s.modelMappings = append(s.modelMappings, modelMappingFromApp(mapping))
	return nil
}

func (s *fakePolicyStore) ListVerdictProviders(ctx context.Context) ([]appcontrol.VerdictProvider, error) {
	if s.listVerdictProvidersErr != nil {
		return nil, s.listVerdictProvidersErr
	}
	items := make([]appcontrol.VerdictProvider, 0, len(s.verdictProviders))
	for _, provider := range s.verdictProviders {
		items = append(items, verdictProviderToApp(provider))
	}
	return items, nil
}

func (s *fakePolicyStore) PutVerdictProvider(ctx context.Context, provider appcontrol.VerdictProvider) error {
	if s.putVerdictProviderErr != nil {
		return s.putVerdictProviderErr
	}
	s.putVerdictProviderCalls++
	s.verdictProviders = append(s.verdictProviders, verdictProviderFromApp(provider))
	return nil
}

func (s *fakePolicyStore) ListGatewayClients(ctx context.Context) ([]appcontrol.GatewayClient, error) {
	if s.listGatewayClientsErr != nil {
		return nil, s.listGatewayClientsErr
	}
	items := make([]appcontrol.GatewayClient, 0, len(s.gatewayClients))
	for _, client := range s.gatewayClients {
		items = append(items, gatewayClientToApp(client))
	}
	return items, nil
}

func (s *fakePolicyStore) CreateGatewayClient(ctx context.Context, request appcontrol.CreateGatewayClientRequest) (appcontrol.CreateGatewayClientResponse, error) {
	if s.createGatewayClientErr != nil {
		return appcontrol.CreateGatewayClientResponse{}, s.createGatewayClientErr
	}
	if request.ID == "" {
		request.ID = "client-test"
	}
	client := gatewayClientDTO{
		ID:        request.ID,
		Name:      request.Name,
		Status:    request.Status,
		KeyPrefix: "kgc_" + request.ID,
	}
	s.gatewayClients = append(s.gatewayClients, client)
	return appcontrol.CreateGatewayClientResponse{Client: gatewayClientToApp(client), Key: "kgc_" + request.ID + ".secret"}, nil
}

func (s *fakePolicyStore) PatchGatewayClient(ctx context.Context, client appcontrol.GatewayClient) (appcontrol.GatewayClient, error) {
	if s.patchGatewayClientErr != nil {
		return appcontrol.GatewayClient{}, s.patchGatewayClientErr
	}
	s.gatewayClients = append(s.gatewayClients, gatewayClientFromApp(client))
	return client, nil
}

func (s *fakePolicyStore) RevokeGatewayClient(ctx context.Context, clientID string) (appcontrol.GatewayClient, error) {
	if s.revokeGatewayClientErr != nil {
		return appcontrol.GatewayClient{}, s.revokeGatewayClientErr
	}
	client := gatewayClientDTO{ID: clientID, Status: "revoked"}
	s.gatewayClients = append(s.gatewayClients, client)
	return gatewayClientToApp(client), nil
}

func (s *fakePolicyStore) ListRouteLimits(ctx context.Context) ([]appcontrol.RouteLimit, error) {
	if s.listRouteLimitsErr != nil {
		return nil, s.listRouteLimitsErr
	}
	items := make([]appcontrol.RouteLimit, 0, len(s.routeLimits))
	for _, limit := range s.routeLimits {
		items = append(items, routeLimitToApp(limit))
	}
	return items, nil
}

func (s *fakePolicyStore) PutRouteLimit(ctx context.Context, limit appcontrol.RouteLimit) (appcontrol.RouteLimit, error) {
	if s.putRouteLimitErr != nil {
		return appcontrol.RouteLimit{}, s.putRouteLimitErr
	}
	s.routeLimits = append(s.routeLimits, routeLimitFromApp(limit))
	return limit, nil
}

func (s *fakePolicyStore) ListClientRouteLimits(ctx context.Context, clientID string) ([]appcontrol.ClientRouteLimit, error) {
	if s.listClientRouteLimitsErr != nil {
		return nil, s.listClientRouteLimitsErr
	}
	items := make([]appcontrol.ClientRouteLimit, 0, len(s.clientRouteLimits))
	for _, limit := range s.clientRouteLimits {
		if limit.ClientID == clientID {
			items = append(items, clientRouteLimitToApp(limit))
		}
	}
	return items, nil
}

func (s *fakePolicyStore) PutClientRouteLimit(ctx context.Context, limit appcontrol.ClientRouteLimit) (appcontrol.ClientRouteLimit, error) {
	if s.putClientRouteLimitErr != nil {
		return appcontrol.ClientRouteLimit{}, s.putClientRouteLimitErr
	}
	s.clientRouteLimits = append(s.clientRouteLimits, clientRouteLimitFromApp(limit))
	return limit, nil
}

func (s *fakePolicyStore) DeleteClientRouteLimit(ctx context.Context, clientID, routeKey string) error {
	if s.deleteClientRouteLimitErr != nil {
		return s.deleteClientRouteLimitErr
	}
	filtered := s.clientRouteLimits[:0]
	for _, limit := range s.clientRouteLimits {
		if limit.ClientID == clientID && limit.RouteKey == routeKey {
			continue
		}
		filtered = append(filtered, limit)
	}
	s.clientRouteLimits = filtered
	return nil
}

type fakeActivationNotifier struct {
	calls          int
	revisionNumber int64
	err            error
}

func (n *fakeActivationNotifier) NotifyConfigActivated(ctx context.Context, revisionNumber int64) error {
	n.calls++
	n.revisionNumber = revisionNumber
	return n.err
}
