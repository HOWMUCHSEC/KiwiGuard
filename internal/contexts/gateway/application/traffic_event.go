package application

import (
	detection "github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
	traffic "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/domain"
)

// ClassifyTrafficEvent derives business observability fields for a gateway event.
func (LifecycleUseCase) ClassifyTrafficEvent(input TrafficEventClassificationInput) TrafficEventClassification {
	return TrafficEventClassification{
		ErrorType:          trafficErrorType(input),
		BlockReason:        trafficBlockReason(input),
		FallbackTriggered:  input.Decision.ModelSignal.FallbackUsed || input.Verdict.FallbackUsed,
		MatchedSpanCount:   uint32(len(input.Verdict.MatchedSpans) + len(input.Decision.Findings)),
		DetectorCategories: detectorCategories(input.Decision),
		PolicyRuleIDs:      policyRuleIDs(input.Decision),
		Action:             trafficAction(input),
	}
}

func trafficErrorType(input TrafficEventClassificationInput) string {
	if input.TerminationReason != "" && input.TerminationReason != "async_shadow_verdict" {
		switch input.TerminationReason {
		case "read_request_error":
			return "invalid_request"
		case "missing_provider":
			return "missing_route"
		default:
			return input.TerminationReason
		}
	}
	reason := trafficBlockReason(input)
	if reason != "" {
		return reason
	}
	if input.Decision.ModelSignal.Error != "" {
		return "verdict_error"
	}
	return ""
}

func trafficBlockReason(input TrafficEventClassificationInput) string {
	switch input.TerminationReason {
	case "unsupported_redaction", "stream_blocked", "missing_client_key", "invalid_client_key",
		"disabled_client_key", "revoked_client_key", "missing_limit_policy",
		"rate_limit_exceeded", "concurrency_limit_exceeded":
		return input.TerminationReason
	case "":
	default:
		return ""
	}
	if input.Decision.Action != policy.ActionBlock {
		return ""
	}
	switch input.Direction {
	case detection.DirectionInput:
		return "blocked_input"
	case detection.DirectionOutput:
		return "blocked_output"
	default:
		return "blocked"
	}
}

func trafficAction(input TrafficEventClassificationInput) traffic.Action {
	if input.Decision.Action != "" {
		return traffic.Action(input.Decision.Action)
	}
	if input.TerminationReason != "" {
		return traffic.Action(policy.ActionBlock)
	}
	return ""
}

func detectorCategories(decision policy.Decision) []string {
	categories := make([]string, 0, len(decision.Findings))
	for _, finding := range decision.Findings {
		categories = append(categories, finding.Category)
	}
	return categories
}

func policyRuleIDs(decision policy.Decision) []string {
	ruleIDs := make([]string, 0, len(decision.RuleHits))
	for _, hit := range decision.RuleHits {
		ruleIDs = append(ruleIDs, hit.RuleKey)
	}
	return ruleIDs
}
