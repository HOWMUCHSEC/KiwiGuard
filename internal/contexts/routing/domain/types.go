// Package routing defines gateway routing business concepts.
package routing

// ExecutionMode identifies how verdicts are enforced for a route.
type ExecutionMode string

const (
	// ExecutionInline enforces verdict and policy decisions before returning.
	ExecutionInline ExecutionMode = "inline"
	// ExecutionAsyncShadow evaluates verdicts out of band without enforcing them.
	ExecutionAsyncShadow ExecutionMode = "async_shadow"
)

// Action identifies gateway fallback and enforcement actions.
type Action string

const (
	// ActionAllow permits traffic without enforcement.
	ActionAllow Action = "allow"
	// ActionBlock rejects traffic.
	ActionBlock Action = "block"
	// ActionRedact permits traffic after redaction.
	ActionRedact Action = "redact"
	// ActionShadowLog records traffic without changing the response.
	ActionShadowLog Action = "shadow_log"
)

// Route maps an OpenAI-compatible endpoint to an upstream provider.
type Route struct {
	Key                string
	Method             string
	Path               string
	ProviderKey        string
	VerdictProviderKey string
	ModelMapping       ModelMapping
	Execution          ExecutionMode
	Fallback           Action
	RequireClientAuth  bool
}

// ModelMapping controls model names used in requests and event fields.
type ModelMapping struct {
	Requested string
	Mapped    string
	Upstream  string
}

// ResolvedModelMapping is the model tuple selected for one request.
type ResolvedModelMapping struct {
	Requested string
	Mapped    string
	Upstream  string
	Found     bool
}

// Resolve returns the public, policy, and upstream model names for a request.
func (m ModelMapping) Resolve(requested string) ResolvedModelMapping {
	if m.Requested != "" && m.Requested != requested {
		return ResolvedModelMapping{}
	}

	mapped := m.Mapped
	if mapped == "" {
		mapped = requested
	}

	upstream := mapped
	if m.Upstream != "" && (m.Requested == "" || m.Requested == requested) {
		upstream = m.Upstream
	}
	if upstream == "" {
		upstream = requested
	}
	return ResolvedModelMapping{
		Requested: requested,
		Mapped:    mapped,
		Upstream:  upstream,
		Found:     true,
	}
}
