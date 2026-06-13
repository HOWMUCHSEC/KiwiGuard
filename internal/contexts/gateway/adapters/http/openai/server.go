package openai

import (
	"encoding/json"
	"net/http"
	"time"

	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	verdict "github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

const (
	defaultMaxBodyBytes    = 1 << 20
	defaultUpstreamTimeout = 30 * time.Second
	defaultVerdictTimeout  = 5 * time.Second
)

// server is the OpenAI-compatible HTTP adapter backed by compiled gateway runtime state.
type server struct {
	configRevisionNumber int64
	maxBodyBytes         int64
	upstreamTimeout      time.Duration
	verdictTimeout       time.Duration
	routes               map[routeKey]Route
	providers            map[string]providerConfig
	snapshot             *policy.Snapshot
	verdict              verdict.Provider
	verdictProviders     map[string]verdict.Provider
	events               events.Writer
	auditGate            AuditGate
	rawCapturePolicies   []RawCapturePolicy
	clients              clientRegistry
	limitResolver        limitResolver
	limitState           *limitState
	lifecycle            appgateway.LifecycleUseCase
}

// routeKey identifies one endpoint inside the compiled gateway runtime.
type routeKey struct {
	method string
	path   string
}

// requestMeta carries normalized request metadata through one gateway exchange.
type requestMeta struct {
	requestID            string
	correlationID        string
	start                time.Time
	method               string
	path                 string
	endpointKind         string
	route                Route
	requested            string
	mapped               string
	upstream             string
	requestBody          []byte
	responseBody         []byte
	requestBytes         int64
	responseBytes        int64
	gatewayStatus        uint16
	upstreamTime         time.Duration
	upstreamStatus       uint16
	streamChunks         int
	redactions           int
	termination          string
	partialOutput        bool
	configRevisionNumber int64
	clientID             string
}

// ServeHTTP routes supported OpenAI-compatible endpoints through the gateway transport adapter.
func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/healthz":
		s.handleHealth(w)
	case chatCompletionsPath, responsesPath:
		s.handleOpenAI(w, r)
	default:
		s.handleUnsupported(w, r)
	}
}

func (s *server) handleHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) handleUnsupported(w http.ResponseWriter, r *http.Request) {
	meta := s.newRequestMeta(r, Route{})
	s.emit(r.Context(), meta, decisionResult{
		direction: detection.DirectionInput,
		decision:  policy.Decision{Action: policy.ActionBlock},
	}, "unsupported_path")
	writeOpenAIError(w, http.StatusNotFound, "unsupported_path", "unsupported path")
}
