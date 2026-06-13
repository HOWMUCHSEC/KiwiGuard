package openai

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOpenAIRequestErrorsAndExtractsChatInput(t *testing.T) {
	req, err := parseOpenAIRequest(chatCompletionsPath, []byte(`{
		"model":"gpt-test",
		"messages":[
			{"role":"system","content":[{"type":"text","text":"system"}]},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":{"text":"nested"}}
		]
	}`))
	if err != nil {
		t.Fatalf("parseOpenAIRequest() error = %v", err)
	}
	if req.model != "gpt-test" || req.text != "system\nhello\nnested" {
		t.Fatalf("request = %+v, want extracted model and joined chat text", req)
	}

	if _, err := parseOpenAIRequest("/v1/unknown", []byte(`{}`)); err == nil {
		t.Fatal("parseOpenAIRequest() unsupported path error = nil, want error")
	}
	if _, err := parseOpenAIRequest(chatCompletionsPath, []byte(`{`)); err == nil {
		t.Fatal("parseOpenAIRequest() malformed json error = nil, want error")
	}
}

func TestEncodeOpenAIRequestRewritesModelAndReportsMarshalError(t *testing.T) {
	encoded, err := encodeOpenAIRequest(openAIRequest{body: map[string]any{"model": "requested"}}, "upstream")
	if err != nil {
		t.Fatalf("encodeOpenAIRequest() error = %v", err)
	}
	if !strings.Contains(string(encoded), `"model":"upstream"`) {
		t.Fatalf("encoded request = %s, want upstream model", encoded)
	}

	_, err = encodeOpenAIRequest(openAIRequest{body: map[string]any{"bad": func() {}}}, "upstream")
	if err == nil {
		t.Fatal("encodeOpenAIRequest() error = nil, want marshal error")
	}
}

func TestExtractOpenAIOutputErrorsAndExtractsChatChoices(t *testing.T) {
	output, err := extractOpenAIOutput(chatCompletionsPath, []byte(`{
		"choices":[
			{"message":{"content":"first"}},
			{"message":{"content":[{"text":"second"}]}},
			{"delta":{"content":"ignored"}}
		]
	}`))
	if err != nil {
		t.Fatalf("extractOpenAIOutput() error = %v", err)
	}
	if output != "first\nsecond" {
		t.Fatalf("extractOpenAIOutput() = %q, want joined chat output", output)
	}

	if _, err := extractOpenAIOutput("/v1/unknown", []byte(`{}`)); err == nil {
		t.Fatal("extractOpenAIOutput() unsupported path error = nil, want error")
	}
	if _, err := extractOpenAIOutput(chatCompletionsPath, []byte(`{`)); err == nil {
		t.Fatal("extractOpenAIOutput() malformed json error = nil, want error")
	}
}

func TestExtractChatStreamDeltaSkipsMalformedChoices(t *testing.T) {
	decoded := map[string]any{
		"choices": []any{
			"bad",
			map[string]any{"message": map[string]any{"content": "ignored"}},
			map[string]any{"delta": map[string]any{"content": []any{"hel", map[string]any{"text": "lo"}}}},
		},
	}

	if got := extractChatStreamDelta(decoded); got != "hello" {
		t.Fatalf("extractChatStreamDelta() = %q, want hello", got)
	}
	if got := extractChatStreamDelta(map[string]any{"choices": "bad"}); got != "" {
		t.Fatalf("extractChatStreamDelta() = %q, want empty for malformed choices", got)
	}
}

func TestExtractResponsesStreamDelta(t *testing.T) {
	tests := []struct {
		name    string
		decoded map[string]any
		want    string
	}{
		{
			name:    "delta string",
			decoded: map[string]any{"type": "response.output_text.delta", "delta": "hel"},
			want:    "hel",
		},
		{
			name:    "text string",
			decoded: map[string]any{"type": "response.output_text.done", "text": "hello"},
			want:    "hello",
		},
		{
			name: "done output item content",
			decoded: map[string]any{
				"type": "response.output_item.done",
				"item": map[string]any{
					"content": []any{
						map[string]any{"type": "output_text", "text": "hello"},
						map[string]any{"type": "refusal", "refusal": "no"},
					},
				},
			},
			want: "hello\nno",
		},
		{
			name: "completed response output",
			decoded: map[string]any{
				"type": "response.completed",
				"response": map[string]any{
					"output": []any{
						map[string]any{
							"content": []any{
								map[string]any{"type": "output_text", "text": "hello"},
							},
						},
					},
				},
			},
			want: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractResponsesStreamDelta(tt.decoded); got != tt.want {
				t.Fatalf("extractResponsesStreamDelta() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractResponsesInput(t *testing.T) {
	decoded := map[string]any{
		"input": []any{
			"plain text",
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "input_text", "text": "nested text"},
					map[string]any{"type": "input_image", "image_url": "https://example.test/image.png"},
				},
			},
			map[string]any{"content": "content string"},
		},
	}

	if got := extractResponsesInput(decoded); got != "plain text\nnested text\ncontent string" {
		t.Fatalf("extractResponsesInput() = %q", got)
	}
}

func TestExtractResponsesOutput(t *testing.T) {
	decoded := map[string]any{
		"output_text": "top-level text",
		"output": []any{
			map[string]any{
				"type": "message",
				"content": []any{
					map[string]any{"type": "output_text", "text": "nested text"},
					map[string]any{"type": "refusal", "refusal": "no"},
				},
			},
			map[string]any{"type": "tool_call", "name": "lookup"},
		},
	}

	if got := extractResponsesOutput(decoded); got != "top-level text\nnested text\nno" {
		t.Fatalf("extractResponsesOutput() = %q", got)
	}
}

func TestAppendTextStringFallback(t *testing.T) {
	parts := appendText(nil, []any{
		map[string]any{"content": map[string]any{"type": "input_text", "text": "nested"}},
		map[string]any{"type": "refusal", "refusal": "fallback"},
		map[string]any{"type": "output_text", "text": ""},
	})

	if len(parts) != 2 || parts[0] != "nested" || parts[1] != "fallback" {
		t.Fatalf("appendText() = %#v, want nested and fallback", parts)
	}
}

func TestParseResponsesStreamDeltaMalformedPayload(t *testing.T) {
	if delta, err := parseOpenAIStreamDelta(responsesPath, "response.output_text.delta", "hello"); err != nil || delta != "hello" {
		t.Fatalf("parseOpenAIStreamDelta() raw delta = %q, err = %v, want hello and nil", delta, err)
	}

	if delta, err := parseOpenAIStreamDelta(responsesPath, "response.completed", "{not-json}"); !errors.Is(err, errInvalidStreamData) || delta != "" {
		t.Fatalf("parseOpenAIStreamDelta() malformed = %q, %v; want invalid stream data", delta, err)
	}

	if delta, err := parseOpenAIStreamDelta(chatCompletionsPath, "", "[DONE]"); err != nil || delta != "" {
		t.Fatalf("parseOpenAIStreamDelta() done = %q, %v; want empty and nil", delta, err)
	}

	if delta, err := parseOpenAIStreamDelta("/v1/unknown", "", `{"delta":"ignored"}`); err != nil || delta != "" {
		t.Fatalf("parseOpenAIStreamDelta() unknown = %q, %v; want empty and nil", delta, err)
	}
}
