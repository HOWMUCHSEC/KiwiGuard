package application

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// RedactionToken is the canonical replacement for unsafe text spans.
const RedactionToken = "[REDACTED]"

// TextSpan identifies a byte range in normalized traffic text.
type TextSpan struct {
	Start int
	End   int
}

// RedactText replaces valid, merged spans with the canonical redaction token.
func RedactText(text string, spans []TextSpan) (string, int, error) {
	if len(spans) == 0 {
		return text, 0, nil
	}
	sort.Slice(spans, func(i, j int) bool {
		return spans[i].Start < spans[j].Start
	})

	merged := make([]TextSpan, 0, len(spans))
	for _, span := range spans {
		if span.Start < 0 || span.End > len(text) || span.Start >= span.End {
			continue
		}
		if !validStringBoundary(text, span.Start) || !validStringBoundary(text, span.End) {
			return "", 0, fmt.Errorf("redact text span: invalid utf-8 boundary")
		}
		if len(merged) == 0 || span.Start > merged[len(merged)-1].End {
			merged = append(merged, span)
			continue
		}
		if span.End > merged[len(merged)-1].End {
			merged[len(merged)-1].End = span.End
		}
	}
	if len(merged) == 0 {
		return text, 0, nil
	}

	var b strings.Builder
	b.Grow(len(text))
	offset := 0
	for _, span := range merged {
		b.WriteString(text[offset:span.Start])
		b.WriteString(RedactionToken)
		offset = span.End
	}
	b.WriteString(text[offset:])
	return b.String(), len(merged), nil
}

func validStringBoundary(text string, offset int) bool {
	return offset == 0 || offset == len(text) || utf8.RuneStart(text[offset])
}
