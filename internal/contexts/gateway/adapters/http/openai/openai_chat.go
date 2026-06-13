package openai

import "strings"

// extractChatStreamDelta extracts streamed assistant text from chat-completions SSE payloads.
func extractChatStreamDelta(decoded map[string]any) string {
	choices, ok := decoded["choices"].([]any)
	if !ok {
		return ""
	}

	parts := make([]string, 0, len(choices))
	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]any)
		if !ok {
			continue
		}
		deltaMap, ok := choiceMap["delta"].(map[string]any)
		if !ok {
			continue
		}
		parts = appendText(parts, deltaMap["content"])
	}
	return strings.Join(parts, "")
}

// extractChatInputSegments normalizes chat input text while preserving editable segments.
func extractChatInputSegments(decoded map[string]any) (string, []textSegment) {
	messages, ok := decoded["messages"].([]any)
	if !ok {
		return "", nil
	}

	var acc textAccumulator
	for _, message := range messages {
		messageMap, ok := message.(map[string]any)
		if !ok {
			continue
		}
		appendTextSegments(&acc, messageMap["content"], func(value string) {
			messageMap["content"] = value
		})
	}
	return acc.text(), acc.segments
}

// extractChatOutputSegments normalizes chat output text while preserving editable segments.
func extractChatOutputSegments(decoded map[string]any) (string, []textSegment) {
	choices, ok := decoded["choices"].([]any)
	if !ok {
		return "", nil
	}

	var acc textAccumulator
	for _, choice := range choices {
		choiceMap, ok := choice.(map[string]any)
		if !ok {
			continue
		}
		messageMap, ok := choiceMap["message"].(map[string]any)
		if !ok {
			continue
		}
		appendTextSegments(&acc, messageMap["content"], func(value string) {
			messageMap["content"] = value
		})
	}
	return acc.text(), acc.segments
}
