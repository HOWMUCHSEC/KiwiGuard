package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestGatewayInvokesVerdictAndForwardsAllowedChatCompletion(t *testing.T) {
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("upstream path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	snapshot := allowSnapshot(t)
	verdictCalls := 0
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		verdictCalls++
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "test"}, nil
	})
	writer := newRecordingWriter()

	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, provider, writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if verdictCalls != 2 {
		t.Fatalf("verdictCalls = %d, want 2", verdictCalls)
	}
	if upstreamBody["model"] != "gpt-upstream" {
		t.Fatalf("upstream model = %v, want gpt-upstream", upstreamBody["model"])
	}
	if len(writer.events()) != 2 {
		t.Fatalf("event count = %d, want 2", len(writer.events()))
	}
}

func TestGatewayBlocksInputBeforeUpstream(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
	}))
	defer upstream.Close()

	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "pii",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []policy.Rule{{
			Key:          "block-email",
			Enabled:      true,
			Severity:     policy.SeverityHigh,
			Action:       policy.ActionBlock,
			DetectorKeys: []string{"email"},
			Scope:        policy.Scope{RouteKey: "openai", Direction: detection.DirectionInput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"alice@example.com"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	event := gotEvents[0]
	if event.GatewayStatus != http.StatusForbidden {
		t.Fatalf("GatewayStatus = %d, want 403", event.GatewayStatus)
	}
	if event.BlockReason != "blocked_input" || event.ErrorType != "blocked_input" {
		t.Fatalf("block fields = %q/%q, want blocked_input", event.BlockReason, event.ErrorType)
	}
	if event.MatchedSpanCount == 0 || event.DetectorLatency <= 0 || event.VerdictLatency <= 0 {
		t.Fatalf("observability fields = spans %d detector %v verdict %v, want populated", event.MatchedSpanCount, event.DetectorLatency, event.VerdictLatency)
	}
}

func TestGatewayUsesRouteSpecificVerdictProvider(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	cfg.Routes[0].VerdictProviderKey = "route-blocker"
	cfg.VerdictProviders = map[string]verdict.Provider{
		"route-blocker": verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
			return verdict.Result{
				SuggestedAction: verdict.ActionBlock,
				RiskLevel:       verdict.RiskHigh,
				ProviderName:    "route-blocker",
			}, nil
		}),
	}
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
}

func TestGatewayHealthEndpointReportsLivenessWithoutAuditEvent(t *testing.T) {
	writer := newRecordingWriter()
	handler := testGateway(t, "http://example.test", http.DefaultClient, allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s, want ok status", rec.Body.String())
	}
	if len(writer.events()) != 0 {
		t.Fatalf("event count = %d, want no audit event for health", len(writer.events()))
	}
}

func TestGatewayRedactsInputBeforeUpstream(t *testing.T) {
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		upstreamBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "pii",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []policy.Rule{{
			Key:          "redact-email",
			Enabled:      true,
			Severity:     policy.SeverityHigh,
			Action:       policy.ActionRedact,
			DetectorKeys: []string{"email"},
			Scope:        policy.Scope{RouteKey: "openai", Direction: detection.DirectionInput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, allowVerdictProvider(), newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"alice@example.com"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(upstreamBody, "alice@example.com") {
		t.Fatalf("upstream body leaked unredacted input: %s", upstreamBody)
	}
	if !strings.Contains(upstreamBody, "[REDACTED]") {
		t.Fatalf("upstream body = %s, want redaction marker", upstreamBody)
	}
}

func TestGatewayRedactsInputFromVerdictMatchedSpans(t *testing.T) {
	var upstreamBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll() error = %v", err)
		}
		upstreamBody = string(body)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		if req.Direction != verdict.DirectionInput {
			return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "verdict"}, nil
		}
		start := strings.Index(req.Text, "sk-test-secret")
		return verdict.Result{
			SuggestedAction: verdict.ActionRedact,
			RiskLevel:       verdict.RiskHigh,
			ProviderName:    "verdict",
			MatchedSpans: []verdict.MatchedSpan{{
				Start:    start,
				End:      start + len("sk-test-secret"),
				Category: "secret.token",
			}},
		}, nil
	})

	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), provider, newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"token sk-test-secret"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(upstreamBody, "sk-test-secret") {
		t.Fatalf("upstream body leaked unredacted verdict span: %s", upstreamBody)
	}
	if !strings.Contains(upstreamBody, "[REDACTED]") {
		t.Fatalf("upstream body = %s, want redaction marker", upstreamBody)
	}
}

func TestGatewayRedactsOutputBeforeClient(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "contact alice@example.com"},
			}},
		})
	}))
	defer upstream.Close()

	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "pii",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []policy.Rule{{
			Key:          "redact-email",
			Enabled:      true,
			Severity:     policy.SeverityHigh,
			Action:       policy.ActionRedact,
			DetectorKeys: []string{"email"},
			Scope:        policy.Scope{RouteKey: "openai", Direction: detection.DirectionOutput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, allowVerdictProvider(), newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "alice@example.com") {
		t.Fatalf("response leaked unredacted output: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "[REDACTED]") {
		t.Fatalf("response body = %s, want redaction marker", rec.Body.String())
	}
}

func TestGatewayDropsStaleContentLengthAfterOutputRedaction(t *testing.T) {
	response := `{"id":"chatcmpl-test","choices":[{"message":{"role":"assistant","content":"contact alice@example.com"}}]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(response)))
		_, _ = io.WriteString(w, response)
	}))
	defer upstream.Close()

	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "pii",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []policy.Rule{{
			Key:          "redact-email",
			Enabled:      true,
			Severity:     policy.SeverityHigh,
			Action:       policy.ActionRedact,
			DetectorKeys: []string{"email"},
			Scope:        policy.Scope{RouteKey: "openai", Direction: detection.DirectionOutput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, allowVerdictProvider(), newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Length"); got != "" && got != strconv.Itoa(rec.Body.Len()) {
		t.Fatalf("Content-Length = %q, want empty or %d after redaction", got, rec.Body.Len())
	}
}

func TestGatewayUnsupportedPathReturnsOpenAIError(t *testing.T) {
	handler := testGateway(t, "http://example.test", http.DefaultClient, allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/unknown", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusNotFound, "unsupported_path")
}

func TestGatewayUnsupportedMethodReturnsOpenAIError(t *testing.T) {
	writer := newRecordingWriter()
	handler := testGateway(t, "http://example.test", http.DefaultClient, allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusNotFound, "unsupported_path")
	if len(writer.events()) != 1 {
		t.Fatalf("event count = %d, want 1", len(writer.events()))
	}
}

func TestGatewayRejectsUnmappedRequestedModel(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-other","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusNotFound, "missing_model_mapping")
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
	if len(writer.events()) != 1 {
		t.Fatalf("event count = %d, want 1", len(writer.events()))
	}
	if writer.events()[0].BlockReason != "" {
		t.Fatalf("BlockReason = %q, want empty for operational mapping failure", writer.events()[0].BlockReason)
	}
}

func TestGatewayRejectsOversizedBodyBeforeUpstream(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	cfg.MaxBodyBytes = 8
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusRequestEntityTooLarge, "request_too_large")
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
}

func TestRawCapturePayloadsRespectRedactionMode(t *testing.T) {
	meta := requestMeta{
		requestBody:  []byte(`{"prompt":"secret"}`),
		responseBody: []byte(`{"answer":"secret"}`),
	}

	tests := []struct {
		name         string
		mode         string
		wantRequest  string
		wantResponse string
	}{
		{name: "default redacts", mode: "", wantRequest: "[redacted]", wantResponse: "[redacted]"},
		{name: "explicit redacted", mode: "redacted", wantRequest: "[redacted]", wantResponse: "[redacted]"},
		{name: "metadata only omits payloads", mode: "metadata_only", wantRequest: "", wantResponse: ""},
		{name: "none preserves payloads", mode: "none", wantRequest: `{"prompt":"secret"}`, wantResponse: `{"answer":"secret"}`},
		{name: "unknown omits payloads", mode: "hash_only", wantRequest: "", wantResponse: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRequest, gotResponse := rawCapturePayloads(RawCapturePolicy{RedactionMode: tt.mode}, meta)
			if gotRequest != tt.wantRequest || gotResponse != tt.wantResponse {
				t.Fatalf("rawCapturePayloads(%q) = %q/%q, want %q/%q", tt.mode, gotRequest, gotResponse, tt.wantRequest, tt.wantResponse)
			}
		})
	}
}

func TestSampleRawCaptureHandlesBoundaryRatesDeterministically(t *testing.T) {
	if sampleRawCapture("req-1", 0) {
		t.Fatal("sampleRawCapture(rate 0) = true, want false")
	}
	if sampleRawCapture("req-1", -1) {
		t.Fatal("sampleRawCapture(negative rate) = true, want false")
	}
	if !sampleRawCapture("req-1", 1) {
		t.Fatal("sampleRawCapture(rate 1) = false, want true")
	}
	first := sampleRawCapture("req-1", 0.5)
	second := sampleRawCapture("req-1", 0.5)
	if first != second {
		t.Fatalf("sampleRawCapture() = %v then %v, want deterministic decision", first, second)
	}
}

func TestCaptureDirectionMatchesConfiguredAliases(t *testing.T) {
	tests := []struct {
		name       string
		configured string
		direction  detection.Direction
		want       bool
	}{
		{name: "empty matches input", configured: "", direction: detection.DirectionInput, want: true},
		{name: "both matches output", configured: "both", direction: detection.DirectionOutput, want: true},
		{name: "request matches input", configured: "request", direction: detection.DirectionInput, want: true},
		{name: "input rejects output", configured: "input", direction: detection.DirectionOutput, want: false},
		{name: "response matches output", configured: "response", direction: detection.DirectionOutput, want: true},
		{name: "output rejects input", configured: "output", direction: detection.DirectionInput, want: false},
		{name: "unknown rejects", configured: "headers", direction: detection.DirectionInput, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := captureDirectionMatches(tt.configured, tt.direction); got != tt.want {
				t.Fatalf("captureDirectionMatches(%q, %q) = %v, want %v", tt.configured, tt.direction, got, tt.want)
			}
		})
	}
}

func TestGatewayModelMappingAliasUsesRoutingResolution(t *testing.T) {
	tests := []struct {
		name         string
		mapping      ModelMapping
		requested    string
		wantMapped   string
		wantUpstream string
		wantOK       bool
	}{
		{
			name:         "explicit mapping",
			mapping:      ModelMapping{Requested: "gpt-public", Mapped: "gpt-policy", Upstream: "gpt-upstream"},
			requested:    "gpt-public",
			wantMapped:   "gpt-policy",
			wantUpstream: "gpt-upstream",
			wantOK:       true,
		},
		{
			name:         "default mapping uses requested",
			mapping:      ModelMapping{},
			requested:    "gpt-public",
			wantMapped:   "gpt-public",
			wantUpstream: "gpt-public",
			wantOK:       true,
		},
		{
			name:      "requested mismatch rejects",
			mapping:   ModelMapping{Requested: "gpt-public", Mapped: "gpt-policy"},
			requested: "gpt-other",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := tt.mapping.Resolve(tt.requested)
			if resolved.Mapped != tt.wantMapped || resolved.Upstream != tt.wantUpstream || resolved.Found != tt.wantOK {
				t.Fatalf("Resolve() = %q/%q/%v, want %q/%q/%v", resolved.Mapped, resolved.Upstream, resolved.Found, tt.wantMapped, tt.wantUpstream, tt.wantOK)
			}
		})
	}
}

func TestPolicyActionAndKnownPolicyAction(t *testing.T) {
	for _, action := range []Action{ActionAllow, ActionBlock, ActionRedact, ActionShadowLog} {
		converted := policyAction(action)
		if converted == "" {
			t.Fatalf("policyAction(%q) = empty, want known policy action", action)
		}
		if !knownPolicyAction(converted) {
			t.Fatalf("knownPolicyAction(%q) = false, want true", converted)
		}
	}
	if got := policyAction(Action("quarantine")); got != "" {
		t.Fatalf("policyAction(unknown) = %q, want empty", got)
	}
	if knownPolicyAction(policy.Action("quarantine")) {
		t.Fatal("knownPolicyAction(unknown) = true, want false")
	}
}

func TestTimeoutAndLatencyHelpers(t *testing.T) {
	base := context.Background()
	ctx, cancel := withTimeout(base, 0)
	defer cancel()
	if ctx != base {
		t.Fatal("withTimeout(0) returned new context, want original context")
	}

	ctx, cancel = withTimeout(base, time.Millisecond)
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("withTimeout() context expired before deadline")
	default:
	}

	if got := verdictLatency(verdict.Result{Latency: 7 * time.Millisecond}, time.Now()); got != 7*time.Millisecond {
		t.Fatalf("verdictLatency() = %v, want explicit latency", got)
	}
	if got := positiveDurationSince(time.Now().Add(time.Second)); got != time.Nanosecond {
		t.Fatalf("positiveDurationSince(future) = %v, want 1ns", got)
	}
}

func TestDecisionFromModelSignalUsesFallbackAndIgnoresUnknownActions(t *testing.T) {
	tests := []struct {
		name        string
		signal      policy.ModelSignal
		wantAction  policy.Action
		wantApplied bool
	}{
		{
			name:        "suggested block applies",
			signal:      policy.ModelSignal{SuggestedAction: policy.ActionBlock},
			wantAction:  policy.ActionBlock,
			wantApplied: true,
		},
		{
			name:        "fallback action wins when fallback was used",
			signal:      policy.ModelSignal{SuggestedAction: policy.ActionAllow, FallbackAction: policy.ActionShadowLog, FallbackUsed: true},
			wantAction:  policy.ActionShadowLog,
			wantApplied: true,
		},
		{
			name:        "unknown action falls back to allow",
			signal:      policy.ModelSignal{SuggestedAction: policy.Action("quarantine")},
			wantAction:  policy.ActionAllow,
			wantApplied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := decisionFromModelSignal(tt.signal)
			if decision.Action != tt.wantAction || decision.ModelSignalApplied != tt.wantApplied {
				t.Fatalf("decision = %+v, want action %q applied %v", decision, tt.wantAction, tt.wantApplied)
			}
			if decision.DefaultAction != policy.ActionAllow {
				t.Fatalf("DefaultAction = %q, want allow", decision.DefaultAction)
			}
		})
	}
}

func TestGatewayLowLevelHelpersCoverFallbackBranches(t *testing.T) {
	if got := endpointKind(chatCompletionsPath); got != "chat_completions" {
		t.Fatalf("endpointKind(chat) = %q, want chat_completions", got)
	}
	if got := endpointKind(responsesPath); got != "responses" {
		t.Fatalf("endpointKind(responses) = %q, want responses", got)
	}
	if got := endpointKind("/v1/unknown"); got != "unknown" {
		t.Fatalf("endpointKind(unknown) = %q, want unknown", got)
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("OpenAI-Request-ID", "openai-id")
	if got := requestIDFromHeader(req); got != "openai-id" {
		t.Fatalf("requestIDFromHeader() = %q, want OpenAI request ID", got)
	}
	req.Header.Set("X-Request-ID", "edge-id")
	if got := requestIDFromHeader(req); got != "edge-id" {
		t.Fatalf("requestIDFromHeader() = %q, want X-Request-ID precedence", got)
	}

	dst := http.Header{}
	src := http.Header{
		"Content-Type":     []string{"application/json"},
		"Connection":       []string{"keep-alive"},
		"X-Upstream-Trace": []string{"trace-1", "trace-2"},
	}
	copyResponseHeaders(dst, src)
	if got := dst.Get("Connection"); got != "" {
		t.Fatalf("copied Connection = %q, want omitted hop-by-hop header", got)
	}
	if got := dst.Values("X-Upstream-Trace"); len(got) != 2 {
		t.Fatalf("X-Upstream-Trace values = %+v, want both values", got)
	}

	body, tooLarge, err := readLimited(strings.NewReader("hello"), 8)
	if err != nil || tooLarge || string(body) != "hello" {
		t.Fatalf("readLimited(ok) = %q/%v/%v, want hello/false/nil", string(body), tooLarge, err)
	}
	body, tooLarge, err = readLimited(strings.NewReader("hello"), 4)
	if err != nil || !tooLarge || body != nil {
		t.Fatalf("readLimited(too large) = %q/%v/%v, want nil/true/nil", string(body), tooLarge, err)
	}
	_, _, err = readLimited(errorReader{}, 4)
	if err == nil || !strings.Contains(err.Error(), "read body") {
		t.Fatalf("readLimited(error) = %v, want wrapped read error", err)
	}
}

func TestWriteSSEFrameTracksBytesAndClientErrors(t *testing.T) {
	srv := &server{}
	meta := &requestMeta{}
	frame := SSEFrame{Raw: []byte("data: hello\n\n")}
	rec := httptest.NewRecorder()

	if !srv.writeSSEFrame(rec, frame, meta) {
		t.Fatal("writeSSEFrame() = false, want true")
	}
	if meta.responseBytes != int64(len(frame.Raw)) {
		t.Fatalf("responseBytes = %d, want %d", meta.responseBytes, len(frame.Raw))
	}

	meta = &requestMeta{}
	if srv.writeSSEFrame(errorResponseWriter{}, frame, meta) {
		t.Fatal("writeSSEFrame(error) = true, want false")
	}
	if meta.termination != "client_write_error" || !meta.partialOutput {
		t.Fatalf("meta = %+v, want client write termination and partial output", meta)
	}
}

func TestJoinURLValidatesBaseAndClearsQuery(t *testing.T) {
	got, err := joinURL("https://gateway.example/base?token=secret", "/v1/chat/completions")
	if err != nil {
		t.Fatalf("joinURL() error = %v", err)
	}
	if got != "https://gateway.example/base/v1/chat/completions" {
		t.Fatalf("joinURL() = %q, want base path joined without query", got)
	}

	if _, err := joinURL("://bad-url", "/v1/chat/completions"); err == nil {
		t.Fatal("joinURL() error = nil, want parse error")
	}
	if _, err := joinURL("/relative", "/v1/chat/completions"); err == nil {
		t.Fatal("joinURL() error = nil, want missing base URL error")
	}
}

func TestWriteSSETermination(t *testing.T) {
	var body strings.Builder
	writeSSETermination(&body, "stream_blocked")

	got := body.String()
	if !strings.Contains(got, `"code":"stream_blocked"`) {
		t.Fatalf("termination frame = %q, want reason code", got)
	}
	if !strings.Contains(got, "data: [DONE]") {
		t.Fatalf("termination frame = %q, want done frame", got)
	}
}

func TestGatewayTimesOutSlowUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	cfg.UpstreamTimeout = 10 * time.Millisecond
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rec, req)

	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("ServeHTTP elapsed = %v, want timeout before upstream response", elapsed)
	}
	assertOpenAIError(t, rec, http.StatusBadGateway, "upstream_error")
}

func TestGatewayAppliesVerdictTimeoutBeforePolicy(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
	}))
	defer upstream.Close()

	sawDeadline := false
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		deadline, ok := ctx.Deadline()
		if ok && time.Until(deadline) <= 100*time.Millisecond {
			sawDeadline = true
		}
		<-ctx.Done()
		return verdict.Result{}, ctx.Err()
	})
	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.VerdictTimeout = 10 * time.Millisecond
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	start := time.Now()
	handler.ServeHTTP(rec, req)

	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("ServeHTTP elapsed = %v, want verdict timeout before upstream", elapsed)
	}
	if !sawDeadline {
		t.Fatal("verdict provider did not receive configured deadline")
	}
	assertOpenAIError(t, rec, http.StatusForbidden, "blocked_input")
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if !gotEvents[0].FallbackTriggered || gotEvents[0].VerdictLatency <= 0 {
		t.Fatalf("fallback fields = triggered %v verdict latency %v, want populated", gotEvents[0].FallbackTriggered, gotEvents[0].VerdictLatency)
	}
}

func TestGatewayDoesNotForwardHopByHopHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, key := range []string{"Connection", "Keep-Alive", "TE", "Trailer", "Transfer-Encoding", "Upgrade", "Proxy-Authorization"} {
			if value := r.Header.Get(key); value != "" {
				t.Fatalf("forwarded %s header = %q", key, value)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("TE", "trailers")
	req.Header.Set("Trailer", "Expires")
	req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Proxy-Authorization", "basic secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestGatewayRejectsProtectedRouteWithoutClientKeyBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, _ := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusUnauthorized, "missing_client_key")
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ErrorType != "missing_client_key" || gotEvents[0].BlockReason != "missing_client_key" || gotEvents[0].ClientID != "" {
		t.Fatalf("event = %+v, want missing client rejection without client id", gotEvents[0])
	}
}

func TestGatewayRejectsDisabledClientBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusDisabled, writer)
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusForbidden, "disabled_client_key")
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ErrorType != "disabled_client_key" || gotEvents[0].BlockReason != "disabled_client_key" {
		t.Fatalf("event = %+v, want disabled client rejection", gotEvents[0])
	}
}

func TestGatewayRejectsRateLimitedClientBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	handler := NewServer(cfg)

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	firstReq.Header.Set("Authorization", "Bearer "+key)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello again"}]}`))
	secondReq.Header.Set("Authorization", "Bearer "+key)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	assertOpenAIError(t, secondRec, http.StatusTooManyRequests, "rate_limit_exceeded")
	if upstreamCalls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	lastEvent := gotEvents[len(gotEvents)-1]
	if lastEvent.ClientID != "client-a" || lastEvent.ErrorType != "rate_limit_exceeded" || lastEvent.BlockReason != "rate_limit_exceeded" {
		t.Fatalf("last event = %+v, want client rate-limit rejection", lastEvent)
	}
}

func TestGatewayStaticConfigProviderPointerPreservesClientLimitState(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	handler := NewServerWithProvider(&StaticConfigProvider{Config: cfg})

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	firstReq.Header.Set("Authorization", "Bearer "+key)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", firstRec.Code, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello again"}]}`))
	secondReq.Header.Set("Authorization", "Bearer "+key)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	assertOpenAIError(t, secondRec, http.StatusTooManyRequests, "rate_limit_exceeded")
	if upstreamCalls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", upstreamCalls.Load())
	}
}

func TestGatewayRejectsConcurrencyLimitedClientBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	enteredUpstream := make(chan struct{}, 1)
	releaseUpstream := make(chan struct{})
	var releaseUpstreamOnce sync.Once
	release := func() {
		releaseUpstreamOnce.Do(func() {
			close(releaseUpstream)
		})
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		enteredUpstream <- struct{}{}
		<-releaseUpstream
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()
	defer release()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	cfg.RouteLimits[0].RequestsPerWindow = 100
	handler := NewServer(cfg)

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hold"}]}`))
	firstReq.Header.Set("Authorization", "Bearer "+key)
	firstRec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.ServeHTTP(firstRec, firstReq)
	}()

	select {
	case <-enteredUpstream:
	case <-time.After(time.Second):
		t.Fatal("first request did not reach upstream")
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"too soon"}]}`))
	secondReq.Header.Set("Authorization", "Bearer "+key)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	assertOpenAIError(t, secondRec, http.StatusTooManyRequests, "concurrency_limit_exceeded")
	if upstreamCalls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", upstreamCalls.Load())
	}

	release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", firstRec.Code, firstRec.Body.String())
	}

	gotEvents := writer.events()
	found := false
	for _, event := range gotEvents {
		if event.ErrorType == "concurrency_limit_exceeded" {
			found = true
			if event.ClientID != "client-a" || event.GatewayStatus != http.StatusTooManyRequests {
				t.Fatalf("concurrency event = %+v, want client 429", event)
			}
		}
	}
	if !found {
		t.Fatalf("events = %+v, want concurrency rejection", gotEvents)
	}
}

func TestGatewayUsesClientBodyLimitBeforeGlobalLimit(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	cfg.MaxBodyBytes = 4096
	cfg.RouteLimits[0].MaxBodyBytes = 80
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"this body is intentionally over the client limit but below the global limit"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusRequestEntityTooLarge, "request_too_large")
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ClientID != "client-a" || gotEvents[0].GatewayStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("event = %+v, want client body-limit rejection", gotEvents[0])
	}
}

func TestGatewayUsesClientBodyLimitBeforeProviderLookup(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	cfg.Routes[0].ProviderKey = "missing"
	cfg.RouteLimits[0].MaxBodyBytes = 80
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"this body is intentionally over the client limit before provider lookup"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusRequestEntityTooLarge, "request_too_large")
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ClientID != "client-a" || gotEvents[0].GatewayStatus != http.StatusRequestEntityTooLarge {
		t.Fatalf("event = %+v, want client body-limit rejection before provider lookup", gotEvents[0])
	}
}

func TestGatewayRejectsProtectedRouteWithoutDefaultLimitBeforeUpstream(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	cfg.Routes[0].RequireClientAuth = true
	cfg.RouteLimits = nil
	cfg.ClientRouteLimitOverrides = []ClientRouteLimitOverride{{
		ClientID:              "client-a",
		RouteKey:              "openai",
		RequestsPerWindow:     10,
		Window:                time.Minute,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          2048,
		Enabled:               true,
	}}
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusForbidden, "missing_limit_policy")
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", upstreamCalls.Load())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ClientID != "client-a" || gotEvents[0].ErrorType != "missing_limit_policy" {
		t.Fatalf("event = %+v, want missing limit policy rejection", gotEvents[0])
	}
}

func TestGatewayForwardsProtectedRequestWithValidClientKey(t *testing.T) {
	var upstreamAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuthorization = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if upstreamAuthorization != "" {
		t.Fatalf("upstream Authorization = %q, want inbound client key stripped", upstreamAuthorization)
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	for _, event := range gotEvents {
		if event.ClientID != "client-a" {
			t.Fatalf("event ClientID = %q, want client-a", event.ClientID)
		}
	}
}

func TestGatewayRejectsInvalidJSON(t *testing.T) {
	writer := newRecordingWriter()
	handler := testGateway(t, "http://example.test", http.DefaultClient, allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusBadRequest, "invalid_json")
	if len(writer.events()) != 1 {
		t.Fatalf("event count = %d, want 1", len(writer.events()))
	}
	if writer.events()[0].BlockReason != "" {
		t.Fatalf("BlockReason = %q, want empty for invalid json", writer.events()[0].BlockReason)
	}
}

func TestGatewayAsyncShadowEmitsVerdictEvent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		return verdict.Result{
			SuggestedAction: verdict.ActionBlock,
			RiskLevel:       verdict.RiskHigh,
			ProviderName:    "shadow",
			Categories:      []string{"shadow.risk"},
			Confidence:      0.91,
		}, nil
	})
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.Routes[0].Execution = ExecutionAsyncShadow
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	gotEvents := waitForEventCount(t, writer, 4)
	if len(gotEvents) != 4 {
		t.Fatalf("event count = %d, want 4: %+v", len(gotEvents), gotEvents)
	}
	shadowEvents := 0
	for _, event := range gotEvents {
		if event.VerdictProviderID == "shadow" {
			shadowEvents++
			if event.Action != events.Action(policy.ActionBlock) {
				t.Fatalf("shadow event action = %q, want block", event.Action)
			}
		}
	}
	if shadowEvents != 2 {
		t.Fatalf("shadow event count = %d, want 2", shadowEvents)
	}
}

func TestGatewayAsyncShadowTimeoutEmitsEventWithLiveContext(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := &contextRejectingWriter{}
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		<-ctx.Done()
		return verdict.Result{}, ctx.Err()
	})
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.Routes[0].Execution = ExecutionAsyncShadow
	cfg.VerdictTimeout = 10 * time.Millisecond
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	gotEvents := waitForContextRejectingEventCount(t, writer, 4)
	if len(gotEvents) != 4 {
		t.Fatalf("event count = %d, want 4: %+v", len(gotEvents), gotEvents)
	}
	shadowEvents := 0
	for _, event := range gotEvents {
		if event.TerminationReason == "async_shadow_verdict" {
			shadowEvents++
			if event.Action != events.Action(policy.ActionBlock) {
				t.Fatalf("shadow timeout action = %q, want block", event.Action)
			}
		}
	}
	if shadowEvents != 2 {
		t.Fatalf("shadow event count = %d, want 2: %+v", shadowEvents, gotEvents)
	}
}

func TestGatewayAsyncShadowTimeoutDoesNotWaitForNonCooperativeVerdict(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	release := make(chan struct{})
	defer close(release)
	writer := newRecordingWriter()
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		<-release
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "late"}, nil
	})
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.Routes[0].Execution = ExecutionAsyncShadow
	cfg.VerdictTimeout = 10 * time.Millisecond
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	gotEvents := waitForEventCount(t, writer, 4)
	if len(gotEvents) != 4 {
		t.Fatalf("event count = %d, want 4: %+v", len(gotEvents), gotEvents)
	}
	shadowEvents := 0
	for _, event := range gotEvents {
		if event.TerminationReason == "async_shadow_verdict" {
			shadowEvents++
		}
	}
	if shadowEvents != 2 {
		t.Fatalf("shadow event count = %d, want 2: %+v", shadowEvents, gotEvents)
	}
}

func TestGatewayBlocksOutputAfterUpstreamAndEmitsEvent(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "blocked response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	calls := 0
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		calls++
		if req.Direction == verdict.DirectionOutput {
			return verdict.Result{SuggestedAction: verdict.ActionBlock, RiskLevel: verdict.RiskHigh, ProviderName: "test"}, nil
		}
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "test"}, nil
	})
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if !upstreamCalled {
		t.Fatal("upstream was not called")
	}
	if calls != 2 {
		t.Fatalf("verdict calls = %d, want 2", calls)
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.Direction != events.Direction("output") {
		t.Fatalf("event direction = %q, want output", outputEvent.Direction)
	}
	if outputEvent.Action != events.Action("block") {
		t.Fatalf("event action = %q, want block", outputEvent.Action)
	}
	if outputEvent.GatewayStatus != http.StatusForbidden || outputEvent.UpstreamStatus != http.StatusOK {
		t.Fatalf("status fields = gateway %d upstream %d, want 403/200", outputEvent.GatewayStatus, outputEvent.UpstreamStatus)
	}
	if outputEvent.BlockReason != "blocked_output" || outputEvent.ErrorType != "blocked_output" {
		t.Fatalf("block fields = %q/%q, want blocked_output", outputEvent.BlockReason, outputEvent.ErrorType)
	}
}

func TestGatewayEventsCaptureAllowedAndUpstreamErrorStatuses(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	for i, event := range gotEvents {
		if event.GatewayStatus != http.StatusOK {
			t.Fatalf("event %d GatewayStatus = %d, want 200", i, event.GatewayStatus)
		}
		if event.VerdictLatency <= 0 || event.DetectorLatency <= 0 {
			t.Fatalf("latency fields = verdict %v detector %v, want populated", event.VerdictLatency, event.DetectorLatency)
		}
	}
	if gotEvents[len(gotEvents)-1].UpstreamStatus != http.StatusOK {
		t.Fatalf("output UpstreamStatus = %d, want 200", gotEvents[len(gotEvents)-1].UpstreamStatus)
	}

	errorUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusTooManyRequests)
	}))
	defer errorUpstream.Close()

	errorWriter := newRecordingWriter()
	errorHandler := testGateway(t, errorUpstream.URL, errorUpstream.Client(), allowSnapshot(t), allowVerdictProvider(), errorWriter)
	errorReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	errorRec := httptest.NewRecorder()
	errorHandler.ServeHTTP(errorRec, errorReq)

	assertOpenAIError(t, errorRec, http.StatusBadGateway, "upstream_error")
	errorEvents := errorWriter.events()
	if len(errorEvents) != 2 {
		t.Fatalf("error event count = %d, want 2", len(errorEvents))
	}
	outputEvent := errorEvents[len(errorEvents)-1]
	if outputEvent.GatewayStatus != http.StatusBadGateway || outputEvent.UpstreamStatus != http.StatusTooManyRequests {
		t.Fatalf("error status fields = gateway %d upstream %d, want 502/429", outputEvent.GatewayStatus, outputEvent.UpstreamStatus)
	}
	if outputEvent.ErrorType != "upstream_error" {
		t.Fatalf("ErrorType = %q, want upstream_error", outputEvent.ErrorType)
	}
	if outputEvent.BlockReason != "" {
		t.Fatalf("BlockReason = %q, want empty for upstream error", outputEvent.BlockReason)
	}
}

func TestGatewayEmitsPayloadsWhenRawCapturePolicyMatches(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.RawCapturePolicies = []RawCapturePolicy{{
		ID:            "capture-openai",
		RouteKey:      "openai",
		Direction:     "both",
		Enabled:       true,
		SampleRate:    1,
		RedactionMode: "none",
	}}
	handler := NewServer(cfg)
	requestBody := `{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	for _, event := range gotEvents {
		if event.RawCapturePolicyID != "capture-openai" {
			t.Fatalf("RawCapturePolicyID = %q, want capture-openai", event.RawCapturePolicyID)
		}
		if event.RequestPayload != requestBody {
			t.Fatalf("RequestPayload = %q, want raw request body", event.RequestPayload)
		}
	}
	if gotEvents[len(gotEvents)-1].ResponsePayload != rec.Body.String() {
		t.Fatalf("ResponsePayload = %q, want gateway response body", gotEvents[len(gotEvents)-1].ResponsePayload)
	}
}

func TestGatewayRedactsPayloadsWhenRawCapturePolicyRequiresRedaction(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.RawCapturePolicies = []RawCapturePolicy{{
		ID:            "redacted-capture",
		RouteKey:      "openai",
		Direction:     "both",
		Enabled:       true,
		SampleRate:    1,
		RedactionMode: "redacted",
	}}
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"alice@example.com"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	for _, event := range writer.events() {
		if event.RawCapturePolicyID != "redacted-capture" {
			t.Fatalf("RawCapturePolicyID = %q, want redacted-capture", event.RawCapturePolicyID)
		}
		if strings.Contains(event.RequestPayload, "alice@example.com") || strings.Contains(event.ResponsePayload, "safe response") {
			t.Fatalf("raw payload leaked under redacted mode: request=%q response=%q", event.RequestPayload, event.ResponsePayload)
		}
	}
}

func TestGatewayMetadataOnlyRawCaptureOmitsPayloadBodies(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.RawCapturePolicies = []RawCapturePolicy{{
		ID:            "metadata-capture",
		RouteKey:      "openai",
		Direction:     "both",
		Enabled:       true,
		SampleRate:    1,
		RedactionMode: "metadata_only",
	}}
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	for _, event := range writer.events() {
		if event.RawCapturePolicyID != "metadata-capture" {
			t.Fatalf("RawCapturePolicyID = %q, want metadata-capture", event.RawCapturePolicyID)
		}
		if event.RequestPayload != "" || event.ResponsePayload != "" {
			t.Fatalf("payloads = %q/%q, want metadata-only payloads omitted", event.RequestPayload, event.ResponsePayload)
		}
	}
}

func TestGatewayOmitsPayloadsWhenRawCapturePolicyDisabledOrMismatched(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	for _, tt := range []struct {
		name   string
		policy RawCapturePolicy
	}{
		{name: "disabled", policy: RawCapturePolicy{ID: "disabled", RouteKey: "openai", Direction: "both", Enabled: false, SampleRate: 1}},
		{name: "mismatched route", policy: RawCapturePolicy{ID: "wrong-route", RouteKey: "admin", Direction: "both", Enabled: true, SampleRate: 1}},
		{name: "zero sample rate", policy: RawCapturePolicy{ID: "sampled-out", RouteKey: "openai", Direction: "both", Enabled: true, SampleRate: 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			writer := newRecordingWriter()
			cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
			cfg.RawCapturePolicies = []RawCapturePolicy{tt.policy}
			handler := NewServer(cfg)
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			for _, event := range writer.events() {
				if event.RawCapturePolicyID != "" || event.RequestPayload != "" || event.ResponsePayload != "" {
					t.Fatalf("event = %+v, want raw payload fields omitted", event)
				}
			}
		})
	}
}

func TestGatewayDoesNotForwardCallerAuthorizationWithoutProviderAPIKey(t *testing.T) {
	var gotAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer caller-secret")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if gotAuthorization != "" {
		t.Fatalf("upstream Authorization = %q, want empty without provider API key", gotAuthorization)
	}
}

func TestGatewayRejectsWhenAuditGateUnhealthyBeforeReadingBodyOrUpstream(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.ConfigRevisionNumber = 42
	cfg.AuditGate = staticAuditGate(false)
	handler := NewServerWithProvider(StaticConfigProvider{Config: cfg})

	body := &trackingReadCloser{reader: strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusServiceUnavailable, "audit_sink_unhealthy")
	if body.read {
		t.Fatal("request body was read before audit health rejection")
	}
	if upstreamCalled {
		t.Fatal("upstream was called")
	}
	gotEvents := writer.events()
	if len(gotEvents) != 1 {
		t.Fatalf("event count = %d, want 1", len(gotEvents))
	}
	if gotEvents[0].ConfigRevisionNumber != 42 {
		t.Fatalf("ConfigRevisionNumber = %d, want 42", gotEvents[0].ConfigRevisionNumber)
	}
}

func TestGatewayRejectsProtectedWhenAuditGateUnhealthyBeforeReadingBodyOrConsumingLimits(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	gate := &mutableAuditGate{healthy: false}
	cfg, key := protectedGatewayConfig(t, upstream.URL, upstream.Client(), ClientStatusEnabled, writer)
	cfg.AuditGate = gate
	handler := NewServer(cfg)

	body := &trackingReadCloser{reader: strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`)}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assertOpenAIError(t, rec, http.StatusServiceUnavailable, "audit_sink_unhealthy")
	if body.read {
		t.Error("request body was read before protected audit health rejection")
	}
	if upstreamCalls.Load() != 0 {
		t.Errorf("upstream calls = %d, want 0", upstreamCalls.Load())
	}

	gate.healthy = true
	nextReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello after recovery"}]}`))
	nextReq.Header.Set("Authorization", "Bearer "+key)
	nextRec := httptest.NewRecorder()
	handler.ServeHTTP(nextRec, nextReq)

	if nextRec.Code != http.StatusOK {
		t.Fatalf("status after audit recovery = %d, want 200; body=%s", nextRec.Code, nextRec.Body.String())
	}
	if upstreamCalls.Load() != 1 {
		t.Fatalf("upstream calls after audit recovery = %d, want 1", upstreamCalls.Load())
	}
}

func TestGatewayForwardsWhenAuditGateHealthy(t *testing.T) {
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-test",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "safe response"},
			}},
		})
	}))
	defer upstream.Close()

	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	cfg.AuditGate = staticAuditGate(true)
	handler := NewServerWithProvider(StaticConfigProvider{Config: cfg})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !upstreamCalled {
		t.Fatal("upstream was not called")
	}
}

func TestGatewayProviderSwapAffectsNewRequests(t *testing.T) {
	firstUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-first",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "first response"},
			}},
		})
	}))
	defer firstUpstream.Close()

	secondUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-second",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "second response"},
			}},
		})
	}))
	defer secondUpstream.Close()

	provider := &swappingConfigProvider{
		cfg: testConfig(firstUpstream.URL, firstUpstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter()),
	}
	provider.cfg.ConfigRevisionNumber = 1
	handler := NewServerWithProvider(provider)

	first := postChatCompletion(handler)
	if !strings.Contains(first.Body.String(), "first response") {
		t.Fatalf("first response = %s, want first upstream", first.Body.String())
	}

	provider.cfg = testConfig(secondUpstream.URL, secondUpstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	provider.cfg.ConfigRevisionNumber = 2
	second := postChatCompletion(handler)
	if !strings.Contains(second.Body.String(), "second response") {
		t.Fatalf("second response = %s, want second upstream", second.Body.String())
	}
}

func TestGatewayProviderServerReusesServerForSameRevision(t *testing.T) {
	firstUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-first",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "first response"},
			}},
		})
	}))
	defer firstUpstream.Close()

	secondUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-second",
			"choices": []map[string]any{{
				"message": map[string]any{"role": "assistant", "content": "second response"},
			}},
		})
	}))
	defer secondUpstream.Close()

	firstCfg := testConfig(firstUpstream.URL, firstUpstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	firstCfg.ConfigRevisionNumber = 7
	secondCfg := testConfig(secondUpstream.URL, secondUpstream.Client(), allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	secondCfg.ConfigRevisionNumber = 7
	provider := &sequenceConfigProvider{configs: []Config{firstCfg, secondCfg}}
	handler := NewServerWithProvider(provider)

	first := postChatCompletion(handler)
	if !strings.Contains(first.Body.String(), "first response") {
		t.Fatalf("first response = %s, want first upstream", first.Body.String())
	}

	second := postChatCompletion(handler)
	if !strings.Contains(second.Body.String(), "first response") {
		t.Fatalf("second response = %s, want cached first upstream", second.Body.String())
	}
}

func testGateway(t *testing.T, upstreamURL string, client *http.Client, snapshot *policy.Snapshot, provider verdict.Provider, writer events.Writer) http.Handler {
	t.Helper()
	return NewServer(testConfig(upstreamURL, client, snapshot, provider, writer))
}

func testConfig(upstreamURL string, client *http.Client, snapshot *policy.Snapshot, provider verdict.Provider, writer events.Writer) Config {
	return Config{
		MaxBodyBytes: 4096,
		Routes: []Route{{
			Key:         "openai",
			Method:      http.MethodPost,
			Path:        "/v1/chat/completions",
			ProviderKey: "upstream",
			ModelMapping: ModelMapping{
				Requested: "gpt-test",
				Mapped:    "gpt-test",
				Upstream:  "gpt-upstream",
			},
			Execution: ExecutionInline,
			Fallback:  ActionBlock,
		}},
		Providers: []Provider{{
			Key:     "upstream",
			BaseURL: upstreamURL,
			Client:  client,
		}},
		Snapshot:        snapshot,
		VerdictProvider: provider,
		EventWriter:     writer,
	}
}

func protectedGatewayConfig(t *testing.T, upstreamURL string, client *http.Client, status ClientStatus, writer events.Writer) (Config, string) {
	t.Helper()
	key, material, err := GenerateClientKey("client-a")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	cfg := testConfig(upstreamURL, client, allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.Clients = []Client{{
		ID:        "client-a",
		Name:      "Client A",
		Status:    status,
		KeyPrefix: material.Prefix,
		KeyHash:   material.Hash,
	}}
	cfg.RouteLimits = []RouteLimitPolicy{{
		RouteKey:              "openai",
		RequestsPerWindow:     1,
		Window:                time.Minute,
		MaxConcurrentRequests: 1,
		MaxBodyBytes:          2048,
		Enabled:               true,
	}}
	return cfg, key
}

func allowSnapshot(t *testing.T) *policy.Snapshot {
	t.Helper()
	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "empty",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}
	return snapshot
}

func allowVerdictProvider() verdict.Provider {
	return verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "test"}, nil
	})
}

func assertOpenAIError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, status, rec.Body.String())
	}

	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v; body=%s", err, rec.Body.String())
	}
	if body.Error.Type == "" {
		t.Fatal("error.type is empty")
	}
	if body.Error.Message == "" {
		t.Fatal("error.message is empty")
	}
	if body.Error.Code != code {
		t.Fatalf("error.code = %q, want %q", body.Error.Code, code)
	}
}

type recordingWriter struct {
	mu    sync.Mutex
	items []events.Event
}

func newRecordingWriter() *recordingWriter {
	return &recordingWriter{}
}

func (w *recordingWriter) Enqueue(ctx context.Context, event events.Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, event)
	return nil
}

func (w *recordingWriter) events() []events.Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]events.Event(nil), w.items...)
}

func waitForEventCount(t *testing.T, writer *recordingWriter, count int) []events.Event {
	t.Helper()
	for range 50 {
		events := writer.events()
		if len(events) >= count {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	return writer.events()
}

type contextRejectingWriter struct {
	mu    sync.Mutex
	items []events.Event
}

func (w *contextRejectingWriter) Enqueue(ctx context.Context, event events.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.items = append(w.items, event)
	return nil
}

func (w *contextRejectingWriter) events() []events.Event {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]events.Event(nil), w.items...)
}

func waitForContextRejectingEventCount(t *testing.T, writer *contextRejectingWriter, count int) []events.Event {
	t.Helper()
	for range 50 {
		events := writer.events()
		if len(events) >= count {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	return writer.events()
}

type staticAuditGate bool

func (g staticAuditGate) Healthy() bool {
	return bool(g)
}

type mutableAuditGate struct {
	healthy bool
}

func (g *mutableAuditGate) Healthy() bool {
	return g.healthy
}

type errorReader struct{}

func (errorReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

type errorResponseWriter struct{}

func (errorResponseWriter) Header() http.Header {
	return http.Header{}
}

func (errorResponseWriter) WriteHeader(statusCode int) {}

func (errorResponseWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

type trackingReadCloser struct {
	reader *strings.Reader
	read   bool
}

func (b *trackingReadCloser) Read(p []byte) (int, error) {
	b.read = true
	return b.reader.Read(p)
}

func (b *trackingReadCloser) Close() error {
	return nil
}

type swappingConfigProvider struct {
	cfg Config
}

func (p *swappingConfigProvider) CurrentConfig() Config {
	return p.cfg
}

type sequenceConfigProvider struct {
	mu      sync.Mutex
	configs []Config
	next    int
}

func (p *sequenceConfigProvider) CurrentConfig() Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.next >= len(p.configs) {
		return p.configs[len(p.configs)-1]
	}
	cfg := p.configs[p.next]
	p.next++
	return cfg
}

func postChatCompletion(handler http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
