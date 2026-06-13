package openai

import "strings"

// textSegment records a text slice and its absolute byte range in the original payload.
type textSegment struct {
	start int
	end   int
	text  string
	apply func(string)
}

// textAccumulator tracks normalized text while preserving original byte spans for redaction.
type textAccumulator struct {
	parts    []string
	segments []textSegment
	length   int
}

func appendText(parts []string, value any) []string {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			parts = append(parts, typed)
		}
	case []any:
		for _, item := range typed {
			parts = appendText(parts, item)
		}
	case map[string]any:
		if text := stringValue(typed["text"]); text != "" {
			parts = append(parts, text)
			return parts
		}
		parts = appendText(parts, typed["content"])
		parts = appendText(parts, typed["output"])
		parts = appendText(parts, typed["item"])
		parts = appendText(parts, typed["response"])
		if text := stringValue(typed["refusal"]); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

func appendTextSegments(acc *textAccumulator, value any, set func(string)) {
	switch typed := value.(type) {
	case string:
		acc.add(typed, set)
	case []any:
		for i := range typed {
			index := i
			appendTextSegments(acc, typed[index], func(updated string) {
				typed[index] = updated
			})
		}
	case map[string]any:
		if _, ok := typed["text"]; ok {
			appendTextSegments(acc, typed["text"], func(updated string) {
				typed["text"] = updated
			})
			return
		}
		appendTextSegments(acc, typed["content"], func(updated string) {
			typed["content"] = updated
		})
		appendTextSegments(acc, typed["output"], func(updated string) {
			typed["output"] = updated
		})
		appendTextSegments(acc, typed["item"], func(updated string) {
			typed["item"] = updated
		})
		appendTextSegments(acc, typed["response"], func(updated string) {
			typed["response"] = updated
		})
		if _, ok := typed["refusal"]; ok {
			appendTextSegments(acc, typed["refusal"], func(updated string) {
				typed["refusal"] = updated
			})
		}
	}
}

func (a *textAccumulator) add(text string, apply func(string)) {
	if text == "" {
		return
	}
	if len(a.parts) > 0 {
		a.length++
	}
	start := a.length
	a.parts = append(a.parts, text)
	a.length += len(text)
	a.segments = append(a.segments, textSegment{
		start: start,
		end:   a.length,
		text:  text,
		apply: apply,
	})
}

func (a textAccumulator) text() string {
	return strings.Join(a.parts, "\n")
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
