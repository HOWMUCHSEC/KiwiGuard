package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// errInvalidStreamData reports malformed transport-specific stream payloads.
var errInvalidStreamData = errors.New("invalid stream data")

const (
	chatCompletionsPath = "/v1/chat/completions"
	responsesPath       = "/v1/responses"
)

// openAIRequest is the decoded request shape needed by the transport adapter.
type openAIRequest struct {
	body     map[string]any
	model    string
	text     string
	segments []textSegment
}

// openAIResponse is the decoded response shape needed by the transport adapter.
type openAIResponse struct {
	body     map[string]any
	text     string
	segments []textSegment
}

// parseOpenAIRequest decodes a supported OpenAI-compatible request body.
func parseOpenAIRequest(path string, body []byte) (openAIRequest, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return openAIRequest{}, fmt.Errorf("parse openai request: %w", err)
	}

	req := openAIRequest{
		body:  decoded,
		model: stringValue(decoded["model"]),
	}
	switch path {
	case chatCompletionsPath:
		req.text, req.segments = extractChatInputSegments(decoded)
	case responsesPath:
		req.text, req.segments = extractResponsesInputSegments(decoded)
	default:
		return openAIRequest{}, fmt.Errorf("parse openai request: unsupported path %s", path)
	}
	return req, nil
}

// encodeOpenAIRequest re-encodes a decoded request with the selected upstream model.
func encodeOpenAIRequest(req openAIRequest, upstreamModel string) ([]byte, error) {
	req.body["model"] = upstreamModel
	encoded, err := json.Marshal(req.body)
	if err != nil {
		return nil, fmt.Errorf("encode openai request: %w", err)
	}
	return encoded, nil
}

// extractOpenAIOutput extracts normalized response text from a supported protocol body.
func extractOpenAIOutput(path string, body []byte) (string, error) {
	response, err := parseOpenAIResponse(path, body)
	if err != nil {
		return "", err
	}
	return response.text, nil
}

func parseOpenAIResponse(path string, body []byte) (openAIResponse, error) {
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return openAIResponse{}, fmt.Errorf("parse openai response: %w", err)
	}

	response := openAIResponse{body: decoded}
	switch path {
	case chatCompletionsPath:
		response.text, response.segments = extractChatOutputSegments(decoded)
	case responsesPath:
		response.text, response.segments = extractResponsesOutputSegments(decoded)
	default:
		return openAIResponse{}, fmt.Errorf("parse openai response: unsupported path %s", path)
	}
	return response, nil
}

func extractOpenAIStreamDelta(path string, data string) string {
	delta, _ := parseOpenAIStreamDelta(path, "", data)
	return delta
}

func parseOpenAIStreamDelta(path string, event string, data string) (string, error) {
	if strings.TrimSpace(data) == "" || strings.TrimSpace(data) == "[DONE]" {
		return "", nil
	}
	if path == responsesPath && strings.HasPrefix(event, "response.output_text.") {
		return data, nil
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(data), &decoded); err != nil {
		return "", errInvalidStreamData
	}

	switch path {
	case chatCompletionsPath:
		return extractChatStreamDelta(decoded), nil
	case responsesPath:
		return extractResponsesStreamDelta(decoded), nil
	default:
		return "", nil
	}
}
