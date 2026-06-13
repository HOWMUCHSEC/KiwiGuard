package openai

import (
	"context"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// emit records one normalized traffic event for the completed exchange.
func (s *server) emit(ctx context.Context, meta requestMeta, result decisionResult, termination string) {
	if s.events == nil {
		return
	}

	classification := s.lifecycle.ClassifyTrafficEvent(appgateway.TrafficEventClassificationInput{
		Direction:         result.direction,
		Decision:          result.decision,
		Verdict:           result.verdict,
		TerminationReason: termination,
	})

	event := traffic.Event{
		EventID:              randomID(),
		SchemaVersion:        "1",
		EventTime:            time.Now(),
		RequestID:            meta.requestID,
		CorrelationID:        meta.correlationID,
		ConfigRevisionNumber: meta.configRevisionNumber,
		SnapshotHash:         result.decision.SnapshotHash,
		ClientID:             meta.clientID,
		RouteID:              meta.route.Key,
		ProviderID:           meta.route.ProviderKey,
		VerdictProviderID:    result.verdict.ProviderName,
		PolicyBundleIDs:      result.decision.BundleKeys,
		HTTPMethod:           meta.method,
		APIPath:              meta.path,
		EndpointKind:         meta.endpointKind,
		RequestedModel:       meta.requested,
		MappedModel:          meta.mapped,
		UpstreamModel:        meta.upstream,
		GatewayStatus:        trafficEventGatewayStatus(meta.gatewayStatus, termination, result.decision),
		UpstreamStatus:       meta.upstreamStatus,
		ErrorType:            classification.ErrorType,
		BlockReason:          classification.BlockReason,
		FallbackTriggered:    classification.FallbackTriggered,
		Direction:            traffic.Direction(result.direction),
		Verdict:              string(result.verdict.SuggestedAction),
		Action:               classification.Action,
		RiskLevel:            string(result.verdict.RiskLevel),
		Categories:           append([]string(nil), result.verdict.Categories...),
		Confidence:           result.verdict.Confidence,
		DetectorCategories:   classification.DetectorCategories,
		MatchedSpanCount:     classification.MatchedSpanCount,
		PolicyRuleIDs:        classification.PolicyRuleIDs,
		RequestBytes:         meta.requestBytes,
		ResponseBytes:        meta.responseBytes,
		GatewayLatency:       time.Since(meta.start),
		DetectorLatency:      result.detectorLatency,
		VerdictLatency:       result.verdictLatency,
		UpstreamLatency:      meta.upstreamTime,
		TotalLatency:         time.Since(meta.start),
		StreamingChunkCount:  meta.streamChunks,
		RedactionCount:       meta.redactions,
		TerminationReason:    firstNonEmpty(termination, meta.termination),
		PartialOutput:        meta.partialOutput,
		RequestHash:          traffic.HashBody(meta.requestBody),
		ResponseHash:         traffic.HashBody(meta.responseBody),
	}
	if capture, ok := s.rawCapturePolicy(meta, result.direction); ok {
		event.RawCapturePolicyID = capture.ID
		event.RequestPayload, event.ResponsePayload = rawCapturePayloads(capture, meta)
	}
	_ = s.events.Enqueue(ctx, event)
}

func trafficEventGatewayStatus(observed uint16, termination string, decision policy.Decision) uint16 {
	if observed != 0 {
		return observed
	}
	switch termination {
	case "missing_client_key", "invalid_client_key":
		return http.StatusUnauthorized
	case "disabled_client_key", "revoked_client_key", "missing_limit_policy", "unsupported_redaction":
		return http.StatusForbidden
	case "rate_limit_exceeded", "concurrency_limit_exceeded":
		return http.StatusTooManyRequests
	case "invalid_json", "invalid_request", "read_request_error":
		return http.StatusBadRequest
	case "request_too_large":
		return http.StatusRequestEntityTooLarge
	case "unsupported_path", "missing_model_mapping":
		return http.StatusNotFound
	case "audit_sink_unhealthy":
		return http.StatusServiceUnavailable
	case "missing_provider", "upstream_error", "upstream_response_too_large":
		return http.StatusBadGateway
	}
	if decision.Action == policy.ActionBlock {
		return http.StatusForbidden
	}
	return http.StatusOK
}

func (s *server) rawCapturePolicy(meta requestMeta, direction detection.Direction) (RawCapturePolicy, bool) {
	for _, capture := range s.rawCapturePolicies {
		if !capture.Enabled || !sampleRawCapture(meta.requestID, capture.SampleRate) {
			continue
		}
		if capture.RouteKey != "" && capture.RouteKey != meta.route.Key {
			continue
		}
		if !captureDirectionMatches(capture.Direction, direction) {
			continue
		}
		return capture, true
	}
	return RawCapturePolicy{}, false
}

func sampleRawCapture(requestID string, rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(requestID))
	bucket := float64(hash.Sum64()) / float64(math.MaxUint64)
	return bucket < rate
}

func rawCapturePayloads(capture RawCapturePolicy, meta requestMeta) (string, string) {
	switch strings.ToLower(capture.RedactionMode) {
	case "", "redacted":
		return redactedPayload(meta.requestBody), redactedPayload(meta.responseBody)
	case "metadata_only":
		return "", ""
	case "none":
		return string(meta.requestBody), string(meta.responseBody)
	default:
		return "", ""
	}
}

func redactedPayload(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	return "[redacted]"
}

func captureDirectionMatches(configured string, direction detection.Direction) bool {
	switch strings.ToLower(configured) {
	case "", "both":
		return true
	case "request", "input":
		return direction == detection.DirectionInput
	case "response", "output":
		return direction == detection.DirectionOutput
	default:
		return false
	}
}

func verdictLatency(result verdict.Result, start time.Time) time.Duration {
	if result.Latency > 0 {
		return result.Latency
	}
	return positiveDurationSince(start)
}

func positiveDurationSince(start time.Time) time.Duration {
	elapsed := time.Since(start)
	if elapsed <= 0 {
		return time.Nanosecond
	}
	return elapsed
}
