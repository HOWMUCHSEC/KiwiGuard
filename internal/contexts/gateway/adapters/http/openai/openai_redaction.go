package openai

import (
	"encoding/json"
	"fmt"

	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/domain"
)

// redactOpenAIRequest applies request-body redaction using normalized text spans.
func redactOpenAIRequest(req *openAIRequest, findings []detection.Finding, matchedSpans []verdict.MatchedSpan) (int, error) {
	return redactTextSegments(req.segments, findings, matchedSpans)
}

// redactOpenAIResponse applies response-body redaction using normalized text spans.
func redactOpenAIResponse(path string, body []byte, findings []detection.Finding, matchedSpans []verdict.MatchedSpan) ([]byte, int, error) {
	response, err := parseOpenAIResponse(path, body)
	if err != nil {
		return nil, 0, err
	}
	count, err := redactTextSegments(response.segments, findings, matchedSpans)
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return body, 0, nil
	}
	encoded, err := json.Marshal(response.body)
	if err != nil {
		return nil, 0, fmt.Errorf("encode redacted openai response: %w", err)
	}
	return encoded, count, nil
}

func redactTextSegments(segments []textSegment, findings []detection.Finding, matchedSpans []verdict.MatchedSpan) (int, error) {
	redactions := 0
	for _, segment := range segments {
		spans := redactionSpansForSegment(segment, findings, matchedSpans)
		if len(spans) == 0 {
			continue
		}
		redacted, count, err := appgateway.RedactText(segment.text, spans)
		if err != nil {
			return 0, err
		}
		if count == 0 {
			continue
		}
		segment.apply(redacted)
		redactions += count
	}
	return redactions, nil
}

func redactionSpansForSegment(segment textSegment, findings []detection.Finding, matchedSpans []verdict.MatchedSpan) []appgateway.TextSpan {
	spans := make([]appgateway.TextSpan, 0, len(findings)+len(matchedSpans))
	for _, finding := range findings {
		spans = appendAbsoluteTextSpan(spans, segment, finding.Start, finding.End)
	}
	for _, span := range matchedSpans {
		spans = appendAbsoluteTextSpan(spans, segment, span.Start, span.End)
	}
	return spans
}

func appendAbsoluteTextSpan(spans []appgateway.TextSpan, segment textSegment, absoluteStart int, absoluteEnd int) []appgateway.TextSpan {
	start := max(absoluteStart, segment.start)
	end := min(absoluteEnd, segment.end)
	if start >= end {
		return spans
	}
	return append(spans, appgateway.TextSpan{
		Start: start - segment.start,
		End:   end - segment.start,
	})
}
