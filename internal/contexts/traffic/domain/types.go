// Package traffic defines gateway traffic facts emitted by KiwiGuard.
package traffic

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Direction describes whether an event concerns request or response content.
type Direction string

// Action describes the gateway action selected for an event.
type Action string

// Event captures the typed traffic and security fields emitted by the gateway.
type Event struct {
	EventID              string
	SchemaVersion        string
	EventTime            time.Time
	RequestID            string
	CorrelationID        string
	ConfigRevisionNumber int64
	SnapshotHash         string
	ClientID             string
	RouteID              string
	ProviderID           string
	VerdictProviderID    string
	PolicyBundleIDs      []string
	HTTPMethod           string
	APIPath              string
	EndpointKind         string
	RequestedModel       string
	MappedModel          string
	UpstreamModel        string
	GatewayStatus        uint16
	UpstreamStatus       uint16
	ErrorType            string
	BlockReason          string
	FallbackTriggered    bool
	Direction            Direction
	Verdict              string
	Action               Action
	RiskLevel            string
	Categories           []string
	Confidence           float64
	DetectorCategories   []string
	MatchedSpanCount     uint32
	PolicyRuleIDs        []string
	RequestHash          string
	ResponseHash         string
	RequestPayload       string
	ResponsePayload      string
	RequestBytes         int64
	ResponseBytes        int64
	GatewayLatency       time.Duration
	DetectorLatency      time.Duration
	VerdictLatency       time.Duration
	UpstreamLatency      time.Duration
	QueueDelay           time.Duration
	TotalLatency         time.Duration
	StreamingChunkCount  int
	RedactionCount       int
	TerminationReason    string
	PartialOutput        bool
	RawCapturePolicyID   string
	RetentionPolicyID    string
	CaptureReference     string
	SinkStatus           string
	RetryCount           uint16
	SpoolStatus          string
	Dropped              bool
	DropReason           string
}

// HashBody returns a lowercase SHA-256 hex digest for body content.
func HashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
