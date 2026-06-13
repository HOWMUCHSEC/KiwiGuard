// Package openai provides OpenAI-compatible HTTP gateway handlers.
package openai

import (
	"net/http"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/clients/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/routing/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

const (
	// ExecutionInline preserves the gateway execution mode name during DDD migration.
	ExecutionInline = routing.ExecutionInline
	// ExecutionAsyncShadow preserves the gateway execution mode name during DDD migration.
	ExecutionAsyncShadow = routing.ExecutionAsyncShadow

	// ActionAllow preserves the gateway action name during DDD migration.
	ActionAllow = routing.ActionAllow
	// ActionBlock preserves the gateway action name during DDD migration.
	ActionBlock = routing.ActionBlock
	// ActionRedact preserves the gateway action name during DDD migration.
	ActionRedact = routing.ActionRedact
	// ActionShadowLog preserves the gateway action name during DDD migration.
	ActionShadowLog = routing.ActionShadowLog
)

type (
	// ExecutionMode preserves the gateway execution mode type during DDD migration.
	ExecutionMode = routing.ExecutionMode
	// Action preserves the gateway action type during DDD migration.
	Action = routing.Action
	// Route preserves the gateway route type during DDD migration.
	Route = routing.Route
	// ModelMapping preserves the gateway model mapping type during DDD migration.
	ModelMapping = routing.ModelMapping

	// ClientStatus preserves the gateway client status type during DDD migration.
	ClientStatus = clients.Status
	// Client preserves the gateway client type during DDD migration.
	Client = clients.Client
	// ClientKeyMaterial preserves the gateway client key material type during DDD migration.
	ClientKeyMaterial = clients.KeyMaterial
	// RouteLimitPolicy preserves the gateway route limit type during DDD migration.
	RouteLimitPolicy = clients.RouteLimitPolicy
	// ClientRouteLimitOverride preserves the gateway client route limit override type during DDD migration.
	ClientRouteLimitOverride = clients.RouteLimitOverride
)

const (
	// ClientStatusEnabled preserves the gateway client status name during DDD migration.
	ClientStatusEnabled = clients.StatusEnabled
	// ClientStatusDisabled preserves the gateway client status name during DDD migration.
	ClientStatusDisabled = clients.StatusDisabled
	// ClientStatusRevoked preserves the gateway client status name during DDD migration.
	ClientStatusRevoked = clients.StatusRevoked
)

// Config contains immutable gateway routing and dependency configuration.
type Config struct {
	ConfigRevisionNumber      int64
	MaxBodyBytes              int64
	UpstreamTimeout           time.Duration
	VerdictTimeout            time.Duration
	Routes                    []Route
	Providers                 []Provider
	Snapshot                  *policy.Snapshot
	VerdictProvider           verdict.Provider
	VerdictProviders          map[string]verdict.Provider
	EventWriter               events.Writer
	AuditGate                 AuditGate
	RawCapturePolicies        []RawCapturePolicy
	Clients                   []Client
	RouteLimits               []RouteLimitPolicy
	ClientRouteLimitOverrides []ClientRouteLimitOverride
}

// AuditGate reports whether audit persistence is healthy enough to accept new LLM traffic.
type AuditGate interface {
	Healthy() bool
}

// ConfigProvider returns the active immutable gateway configuration.
type ConfigProvider interface {
	CurrentConfig() Config
}

// StaticConfigProvider adapts a Config into a ConfigProvider.
type StaticConfigProvider struct {
	Config Config
}

// CurrentConfig returns the static gateway configuration.
func (p StaticConfigProvider) CurrentConfig() Config {
	return p.Config
}

// Provider contains upstream provider connection settings.
type Provider struct {
	Key     string
	BaseURL string
	APIKey  string
	Client  *http.Client
	Headers map[string]string
	Timeout time.Duration
}

// RawCapturePolicy controls whether raw request and response payloads are mirrored.
type RawCapturePolicy struct {
	ID            string
	RouteKey      string
	Direction     string
	Enabled       bool
	SampleRate    float64
	RedactionMode string
}
