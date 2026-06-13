package httpapi

// errorResponse is the shared JSON error envelope returned by control-plane endpoints.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type configStatusResponse struct {
	ActivePolicyBundleKeys []string `json:"active_policy_bundle_keys"`
	PolicySnapshotHash     string   `json:"policy_snapshot_hash"`
}

type policyBundleListResponse struct {
	Items []policyBundleDTO `json:"items"`
}

type policyBundleDTO struct {
	Key           string        `json:"key"`
	Version       string        `json:"version"`
	Source        string        `json:"source"`
	DefaultAction string        `json:"default_action"`
	Detectors     []detectorDTO `json:"detectors,omitempty"`
	Rules         []ruleDTO     `json:"rules,omitempty"`
}

type detectorDTO struct {
	Key        string   `json:"key"`
	Kind       string   `json:"kind"`
	Pattern    string   `json:"pattern,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type ruleDTO struct {
	Key          string   `json:"key"`
	Enabled      bool     `json:"enabled"`
	Severity     string   `json:"severity"`
	Action       string   `json:"action"`
	DetectorKeys []string `json:"detector_keys"`
	Scope        scopeDTO `json:"scope,omitempty"`
}

type scopeDTO struct {
	RouteKey  string `json:"route_key,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Model     string `json:"model,omitempty"`
	Direction string `json:"direction,omitempty"`
}

type policyValidationResponse struct {
	Valid bool   `json:"valid"`
	Error string `json:"error,omitempty"`
	Hash  string `json:"hash,omitempty"`
}

type policyActivationRequest struct {
	Keys   []string `json:"keys"`
	Reason string   `json:"reason,omitempty"`
}

type policyActivationResponse struct {
	ActiveKeys        []string `json:"active_keys"`
	Hash              string   `json:"hash"`
	NotificationError string   `json:"notification_error,omitempty"`
	RevisionNumber    int64    `json:"revision_number"`
}

type modelMappingListResponse struct {
	Items []modelMappingDTO `json:"items"`
}

type modelMappingDTO struct {
	ID       string `json:"id"`
	RouteKey string `json:"route_key"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
}

type verdictProviderListResponse struct {
	Items []verdictProviderDTO `json:"items"`
}

type verdictProviderDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint"`
	CredentialRef string `json:"credential_ref,omitempty"`
	Mode          string `json:"mode"`
	Enabled       bool   `json:"enabled"`
}

type gatewayClientListResponse struct {
	Items []gatewayClientDTO `json:"items"`
}

type gatewayClientDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	KeyPrefix string `json:"key_prefix,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

type createGatewayClientRequest struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
	Notes  string `json:"notes,omitempty"`
}

type createGatewayClientResponse struct {
	Client gatewayClientDTO `json:"client"`
	Key    string           `json:"key"`
}

type routeLimitListResponse struct {
	Items []routeLimitDTO `json:"items"`
}

type routeLimitDTO struct {
	RouteKey              string `json:"route_key"`
	RequestsPerWindow     int    `json:"requests_per_window"`
	WindowSeconds         int    `json:"window_seconds"`
	MaxConcurrentRequests int    `json:"max_concurrent_requests"`
	MaxBodyBytes          int64  `json:"max_body_bytes"`
	Enabled               bool   `json:"enabled"`
}

type clientRouteLimitListResponse struct {
	Items []clientRouteLimitDTO `json:"items"`
}

type clientRouteLimitDTO struct {
	ClientID              string `json:"client_id"`
	RouteKey              string `json:"route_key"`
	RequestsPerWindow     int    `json:"requests_per_window"`
	WindowSeconds         int    `json:"window_seconds"`
	MaxConcurrentRequests int    `json:"max_concurrent_requests"`
	MaxBodyBytes          int64  `json:"max_body_bytes"`
	Enabled               bool   `json:"enabled"`
}

type regexTestRequest struct {
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
}

type regexTestResponse struct {
	Matches []regexMatchDTO `json:"matches"`
}

type regexMatchDTO struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

type policyDryRunRequest struct {
	RouteKey  string          `json:"route_key"`
	Provider  string          `json:"provider"`
	Model     string          `json:"model"`
	Direction string          `json:"direction"`
	Text      string          `json:"text"`
	Bundle    policyBundleDTO `json:"bundle"`
}

type policyDryRunResponse struct {
	Decision decisionDTO `json:"decision"`
}

type decisionDTO struct {
	Action             string       `json:"action"`
	DefaultAction      string       `json:"default_action"`
	RuleHits           []ruleHitDTO `json:"rule_hits"`
	Findings           []findingDTO `json:"findings"`
	ModelSignalApplied bool         `json:"model_signal_applied"`
	SnapshotHash       string       `json:"snapshot_hash"`
	BundleKeys         []string     `json:"bundle_keys"`
}

type ruleHitDTO struct {
	BundleKey string       `json:"bundle_key"`
	RuleKey   string       `json:"rule_key"`
	Severity  string       `json:"severity"`
	Action    string       `json:"action"`
	Findings  []findingDTO `json:"findings"`
}

type findingDTO struct {
	DetectorKey string `json:"detector_key"`
	Category    string `json:"category"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
	ValueHash   string `json:"value_hash"`
}
