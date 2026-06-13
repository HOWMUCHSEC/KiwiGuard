package openai

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
)

// pendingSSEFrame keeps one frame buffered until streaming policy checks permit emission.
type pendingSSEFrame struct {
	frame SSEFrame
}

// handleStreaming applies streaming output policy while proxying an SSE upstream response.
func (s *server) handleStreaming(w http.ResponseWriter, r *http.Request, upstreamCtx context.Context, upstreamResp *http.Response, upstreamBody io.Reader, meta requestMeta) {
	copyResponseHeaders(w.Header(), upstreamResp.Header)
	w.Header().Set("Content-Type", "text/event-stream")
	clearStreamingWriteDeadline(w)
	meta.gatewayStatus = uint16(upstreamResp.StatusCode)
	w.WriteHeader(upstreamResp.StatusCode)

	flusher, _ := w.(http.Flusher)
	parser := NewSSEParser(upstreamBody)
	streamBuffer := appgateway.NewStreamBuffer(4096, s.maxBodyBytes)
	planner := appgateway.StreamingOutputUseCase{Lifecycle: s.lifecycle, Buffer: streamBuffer}
	outputDecision := allowedOutputDecisionResult()
	var pending *pendingSSEFrame

	for {
		frame, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			meta.termination = streamingReadTermination(upstreamCtx, err)
			meta.partialOutput = true
			outputDecision = outputBlockedDecisionResult()
			writeSSETermination(w, meta.termination)
			break
		}

		meta.streamChunks++
		delta, err := parseOpenAIStreamDelta(r.URL.Path, frame.Event, frame.Data)
		if err != nil {
			meta.termination = "sse_parse_error"
			meta.partialOutput = true
			outputDecision = outputBlockedDecisionResult()
			writeSSETermination(w, "sse_parse_error")
			break
		}
		if delta != "" {
			plan := planner.PlanDelta(appgateway.StreamingDeltaPlanInput{Text: delta})
			if plan.Action == appgateway.StreamingOutputActionEvaluatePolicy {
				outputDecision = s.evaluateStreamingOutput(r.Context(), meta, plan.PolicyText)
				plan = planner.PlanPolicy(appgateway.StreamingPolicyPlanInput{
					Decision:           outputDecision.decision,
					CollectionAccepted: plan.CollectionAccepted,
				})
			}
			if plan.Action == appgateway.StreamingOutputActionTerminate {
				if plan.ForceBlockedOutput {
					outputDecision = outputBlockedDecisionResult()
				}
				meta.termination = plan.TerminationReason
				meta.partialOutput = true
				writeSSETermination(w, plan.TerminationReason)
				break
			}
		}

		plan := planner.PlanFrame(appgateway.StreamingFramePlanInput{
			HasPending: pending != nil,
			Done:       frame.Done,
		})
		if (plan.Action == appgateway.StreamingOutputActionWritePending || plan.Action == appgateway.StreamingOutputActionComplete) && pending != nil {
			if !s.writeSSEFrame(w, pending.frame, &meta) {
				break
			}
		}
		pending = &pendingSSEFrame{frame: frame}
		if plan.Action == appgateway.StreamingOutputActionComplete {
			if !s.writeSSEFrame(w, pending.frame, &meta) {
				break
			}
			pending = nil
			break
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	if pending != nil && meta.termination == "" {
		_ = s.writeSSEFrame(w, pending.frame, &meta)
	}
	if flusher != nil {
		flusher.Flush()
	}

	meta.responseBody = append([]byte(nil), streamBuffer.CollectedBytes()...)
	plan := planner.PlanFinal(appgateway.StreamingFinalPlanInput{
		PolicyAllowed: outputDecision.allowed(),
		CollectedText: streamBuffer.CollectedText(),
	})
	if plan.Action == appgateway.StreamingOutputActionEvaluatePolicy {
		outputDecision = s.evaluateOutput(r.Context(), meta, plan.PolicyText)
	}
	s.emit(r.Context(), meta, outputDecision, meta.termination)
}

// writeSSEFrame writes one raw SSE frame and updates transport response counters.
func (s *server) writeSSEFrame(w http.ResponseWriter, frame SSEFrame, meta *requestMeta) bool {
	if _, err := w.Write(frame.Raw); err != nil {
		meta.termination = "client_write_error"
		meta.partialOutput = true
		return false
	}
	meta.responseBytes += int64(len(frame.Raw))
	return true
}

// streamingReadTermination classifies streaming read failures into lifecycle termination reasons.
func streamingReadTermination(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return "upstream_timeout"
	}
	return "sse_parse_error"
}

// clearStreamingWriteDeadline removes server write deadlines so upstream streams can stay open.
func clearStreamingWriteDeadline(w http.ResponseWriter) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
}
