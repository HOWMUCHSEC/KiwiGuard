package openai

import "strings"

// extractResponsesStreamDelta extracts streamed model text from Responses API SSE payloads.
func extractResponsesStreamDelta(decoded map[string]any) string {
	parts := appendText(nil, decoded["delta"])
	parts = appendText(parts, decoded["text"])
	parts = appendJoinedText(parts, decoded["item"])
	parts = appendJoinedText(parts, decoded["response"])
	return strings.Join(parts, "")
}

// extractResponsesInput extracts normalized input text from a Responses API request.
func extractResponsesInput(decoded map[string]any) string {
	text, _ := extractResponsesInputSegments(decoded)
	return text
}

// extractResponsesInputSegments normalizes input text while preserving editable segments.
func extractResponsesInputSegments(decoded map[string]any) (string, []textSegment) {
	var acc textAccumulator
	appendTextSegments(&acc, decoded["input"], func(value string) {
		decoded["input"] = value
	})
	return acc.text(), acc.segments
}

// extractResponsesOutput extracts normalized output text from a Responses API response.
func extractResponsesOutput(decoded map[string]any) string {
	text, _ := extractResponsesOutputSegments(decoded)
	return text
}

// extractResponsesOutputSegments normalizes output text while preserving editable segments.
func extractResponsesOutputSegments(decoded map[string]any) (string, []textSegment) {
	var acc textAccumulator
	appendTextSegments(&acc, decoded["output_text"], func(value string) {
		decoded["output_text"] = value
	})

	output, ok := decoded["output"].([]any)
	if !ok {
		return acc.text(), acc.segments
	}
	for _, item := range output {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, ok := itemMap["content"].([]any)
		if !ok {
			continue
		}
		for _, contentItem := range content {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			appendTextSegments(&acc, contentMap, func(value string) {
				contentMap["text"] = value
			})
		}
	}
	return acc.text(), acc.segments
}

func appendJoinedText(parts []string, value any) []string {
	text := strings.Join(appendText(nil, value), "\n")
	if text == "" {
		return parts
	}
	return append(parts, text)
}
