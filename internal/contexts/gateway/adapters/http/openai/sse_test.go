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
	"sync/atomic"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/events"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestSSEParserReadsOpenAIDataFrames(t *testing.T) {
	parser := NewSSEParser(strings.NewReader(": keepalive\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\ndata: [DONE]\n\n"))

	frame, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if frame.Comment != "keepalive" {
		t.Fatalf("Comment = %q, want keepalive", frame.Comment)
	}

	frame, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() second error = %v", err)
	}
	if got := frame.Data; got != `{"choices":[{"delta":{"content":"hel"}}]}` {
		t.Fatalf("Data = %q", got)
	}

	frame, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() third error = %v", err)
	}
	if got := frame.Data; got != `{"choices":[{"delta":{"content":"lo"}}]}` {
		t.Fatalf("Data = %q", got)
	}

	frame, err = parser.Next()
	if err != nil {
		t.Fatalf("Next() done error = %v", err)
	}
	if !frame.Done {
		t.Fatal("Done = false, want true")
	}

	if _, err := parser.Next(); err != io.EOF {
		t.Fatalf("Next() final error = %v, want EOF", err)
	}
}

func TestSSEParserJoinsMultilineDataAndPreservesRaw(t *testing.T) {
	parser := NewSSEParser(strings.NewReader("id: 42\nevent: response.output_text.delta\ndata: hello\ndata: world\nretry: 1000\n\n"))

	frame, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if frame.ID != "42" {
		t.Fatalf("ID = %q, want 42", frame.ID)
	}
	if frame.Event != "response.output_text.delta" {
		t.Fatalf("Event = %q, want response.output_text.delta", frame.Event)
	}
	if frame.Data != "hello\nworld" {
		t.Fatalf("Data = %q, want multiline data", frame.Data)
	}
	if frame.Retry != "1000" {
		t.Fatalf("Retry = %q, want 1000", frame.Retry)
	}
	if string(frame.Raw) != "id: 42\nevent: response.output_text.delta\ndata: hello\ndata: world\nretry: 1000\n\n" {
		t.Fatalf("Raw = %q", string(frame.Raw))
	}
}

func TestSSEParserReadsFinalFrameWithoutTrailingBlankLine(t *testing.T) {
	parser := NewSSEParser(strings.NewReader("data: {\"delta\":\"tail\"}"))

	frame, err := parser.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if frame.Data != `{"delta":"tail"}` {
		t.Fatalf("Data = %q", frame.Data)
	}
	if _, err := parser.Next(); err != io.EOF {
		t.Fatalf("Next() final error = %v, want EOF", err)
	}
}

func BenchmarkSSEParser(b *testing.B) {
	payload := strings.Repeat("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n", 100)

	b.ReportAllocs()
	for b.Loop() {
		parser := NewSSEParser(strings.NewReader(payload))
		for {
			_, err := parser.Next()
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				break
			}
			b.Fatalf("Next() error = %v", err)
		}
	}
}

func TestGatewayPassesAllowedStreamingOutput(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"hel"`) || !strings.Contains(body, `"lo"`) || !strings.Contains(body, "[DONE]") {
		t.Fatalf("stream body missing expected frames: %s", body)
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.StreamingChunkCount != 3 {
		t.Fatalf("StreamingChunkCount = %d, want 3", outputEvent.StreamingChunkCount)
	}
	if outputEvent.PartialOutput {
		t.Fatal("PartialOutput = true, want false")
	}
}

func TestGatewayStreamingInvokesVerdictOnceForFinalOutput(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	var calls atomic.Int32
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		calls.Add(1)
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "test"}, nil
	})
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), provider, newRecordingWriter())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("verdict calls = %d, want input plus final output only", got)
	}
}

func TestGatewayStreamingFinalVerdictUsesCompleteBoundedOutput(t *testing.T) {
	prefix := "stream-start-" + strings.Repeat("a", 4096)
	suffix := "-stream-end"
	fullOutput := prefix + suffix
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(prefix) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(suffix) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	var finalText atomic.Value
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		if req.Direction == verdict.DirectionOutput {
			finalText.Store(req.Text)
		}
		return verdict.Result{SuggestedAction: verdict.ActionAllow, RiskLevel: verdict.RiskLow, ProviderName: "test"}, nil
	})
	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.MaxBodyBytes = int64(len(fullOutput) + 128)
	cfg.RawCapturePolicies = []RawCapturePolicy{{
		ID:            "stream-capture",
		RouteKey:      "openai",
		Direction:     "output",
		Enabled:       true,
		SampleRate:    1,
		RedactionMode: "none",
	}}
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := finalText.Load().(string); got != fullOutput {
		t.Fatalf("final verdict text length = %d, want complete output length %d", len(got), len(fullOutput))
	}
	gotEvents := writer.events()
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.ResponseHash != events.HashBody([]byte(fullOutput)) {
		t.Fatalf("ResponseHash = %q, want hash of complete stream output", outputEvent.ResponseHash)
	}
	if outputEvent.ResponsePayload != fullOutput {
		t.Fatalf("ResponsePayload length = %d, want complete stream output length %d", len(outputEvent.ResponsePayload), len(fullOutput))
	}
}

func TestGatewayTerminatesStreamingOutputWhenCollectedTextExceedsLimit(t *testing.T) {
	firstChunk := strings.Repeat("a", 100)
	oversizedChunk := strings.Repeat("b", 40)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(firstChunk) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":" + strconv.Quote(oversizedChunk) + "}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.MaxBodyBytes = 128
	handler := NewServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), oversizedChunk) {
		t.Fatalf("stream leaked oversized chunk: %s", rec.Body.String())
	}
	outputEvent := writer.events()[1]
	if outputEvent.TerminationReason != "stream_output_too_large" {
		t.Fatalf("TerminationReason = %q, want stream_output_too_large", outputEvent.TerminationReason)
	}
	if outputEvent.Action != events.Action(policy.ActionBlock) {
		t.Fatalf("Action = %q, want block", outputEvent.Action)
	}
}

func TestGatewayTerminatesStreamingOutputWhenPolicyBlocksChunk(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"alice@\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"example.com\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	snapshot, err := policy.CompileSnapshot([]policy.Bundle{{
		Key:           "pii",
		Version:       "1.0.0",
		Source:        policy.SourceBuiltIn,
		DefaultAction: policy.ActionAllow,
		Detectors:     []detection.Definition{{Key: "email", Kind: detection.KindEmail}},
		Rules: []policy.Rule{{
			Key:          "block-output-email",
			Enabled:      true,
			Severity:     policy.SeverityHigh,
			Action:       policy.ActionBlock,
			DetectorKeys: []string{"email"},
			Scope:        policy.Scope{RouteKey: "openai", Direction: detection.DirectionOutput},
		}},
	}})
	if err != nil {
		t.Fatalf("CompileSnapshot() error = %v", err)
	}

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), snapshot, allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "alice@") || strings.Contains(body, "example.com") {
		t.Fatalf("stream leaked blocked email: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatalf("stream missing protocol-safe terminator: %s", body)
	}
	gotEvents := writer.events()
	if len(gotEvents) != 2 {
		t.Fatalf("event count = %d, want 2", len(gotEvents))
	}
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.Action != events.Action(policy.ActionBlock) {
		t.Fatalf("output action = %q, want block", outputEvent.Action)
	}
	if outputEvent.TerminationReason != "stream_blocked" {
		t.Fatalf("TerminationReason = %q, want stream_blocked", outputEvent.TerminationReason)
	}
	if !outputEvent.PartialOutput {
		t.Fatal("PartialOutput = false, want true")
	}
}

func TestGatewayTerminatesMalformedStreamingDataFrame(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {not-json}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	handler := testGateway(t, upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "not-json") {
		t.Fatalf("stream leaked malformed frame: %s", body)
	}
	if !strings.Contains(body, "[DONE]") {
		t.Fatalf("stream missing protocol-safe terminator: %s", body)
	}
	gotEvents := writer.events()
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.TerminationReason != "sse_parse_error" {
		t.Fatalf("TerminationReason = %q, want sse_parse_error", outputEvent.TerminationReason)
	}
}

func TestGatewayTerminatesStreamingOutputOnUpstreamTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"held\"}}]}\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), allowVerdictProvider(), writer)
	cfg.UpstreamTimeout = 10 * time.Millisecond
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "held") {
		t.Fatalf("stream leaked pending timed-out chunk: %s", body)
	}
	if !strings.Contains(body, `"upstream_timeout"`) || !strings.Contains(body, "[DONE]") {
		t.Fatalf("stream missing upstream timeout terminator: %s", body)
	}
	gotEvents := writer.events()
	outputEvent := gotEvents[len(gotEvents)-1]
	if outputEvent.TerminationReason != "upstream_timeout" {
		t.Fatalf("TerminationReason = %q, want upstream_timeout", outputEvent.TerminationReason)
	}
	if !outputEvent.PartialOutput {
		t.Fatal("PartialOutput = false, want true")
	}
}

func TestHandleStreamingClearsResponseWriteDeadline(t *testing.T) {
	writer := &deadlineRecordingResponseWriter{}
	cfg := testConfig("http://upstream.test", http.DefaultClient, allowSnapshot(t), allowVerdictProvider(), newRecordingWriter())
	s := newServer(cfg)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	meta := s.newRequestMeta(req, cfg.Routes[0])
	upstreamResp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	s.handleStreaming(writer, req, context.Background(), upstreamResp, strings.NewReader("data: [DONE]\n\n"), meta)

	if len(writer.writeDeadlines) != 1 {
		t.Fatalf("write deadline count = %d, want 1", len(writer.writeDeadlines))
	}
	if !writer.writeDeadlines[0].IsZero() {
		t.Fatalf("write deadline = %v, want zero time", writer.writeDeadlines[0])
	}
}

func TestGatewayStreamingAsyncShadowDoesNotBlockOutput(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"safe\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	writer := newRecordingWriter()
	provider := verdict.ProviderFunc(func(ctx context.Context, req verdict.Request) (verdict.Result, error) {
		return verdict.Result{SuggestedAction: verdict.ActionBlock, RiskLevel: verdict.RiskHigh, ProviderName: "shadow"}, nil
	})
	cfg := testConfig(upstream.URL, upstream.Client(), allowSnapshot(t), provider, writer)
	cfg.Routes[0].Execution = ExecutionAsyncShadow
	handler := NewServer(cfg)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","stream":true,"messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"safe"`) {
		t.Fatalf("stream body missing safe output: %s", rec.Body.String())
	}
	gotEvents := waitForEventCount(t, writer, 4)
	shadowEvents := 0
	for _, event := range gotEvents {
		if event.VerdictProviderID == "shadow" {
			shadowEvents++
		}
	}
	if shadowEvents != 2 {
		t.Fatalf("shadow event count = %d, want 2: %+v", shadowEvents, gotEvents)
	}
}

type deadlineRecordingResponseWriter struct {
	header         http.Header
	status         int
	body           strings.Builder
	writeDeadlines []time.Time
}

func (w *deadlineRecordingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *deadlineRecordingResponseWriter) Write(body []byte) (int, error) {
	return w.body.Write(body)
}

func (w *deadlineRecordingResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *deadlineRecordingResponseWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadlines = append(w.writeDeadlines, deadline)
	return nil
}

func TestExtractOpenAIStreamDelta(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"choices": []map[string]any{{
			"delta": map[string]any{"content": "hello"},
		}},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got := extractOpenAIStreamDelta(chatCompletionsPath, string(payload)); got != "hello" {
		t.Fatalf("delta = %q, want hello", got)
	}
}
