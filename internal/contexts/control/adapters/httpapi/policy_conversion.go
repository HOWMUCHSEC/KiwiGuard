package httpapi

import (
	"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// toDecisionDTO converts a domain policy decision into the control API response shape.
func toDecisionDTO(decision policy.Decision) decisionDTO {
	ruleHits := make([]ruleHitDTO, 0, len(decision.RuleHits))
	for _, hit := range decision.RuleHits {
		ruleHits = append(ruleHits, ruleHitDTO{
			BundleKey: string(hit.BundleKey),
			RuleKey:   hit.RuleKey,
			Severity:  string(hit.Severity),
			Action:    string(hit.Action),
			Findings:  toFindingDTOs(hit.Findings),
		})
	}

	return decisionDTO{
		Action:             string(decision.Action),
		DefaultAction:      string(decision.DefaultAction),
		RuleHits:           ruleHits,
		Findings:           toFindingDTOs(decision.Findings),
		ModelSignalApplied: decision.ModelSignalApplied,
		SnapshotHash:       decision.SnapshotHash,
		BundleKeys:         append([]string(nil), decision.BundleKeys...),
	}
}

// toFindingDTOs converts detector findings into control API response items.
func toFindingDTOs(findings []detection.Finding) []findingDTO {
	items := make([]findingDTO, 0, len(findings))
	for _, finding := range findings {
		items = append(items, findingDTO{
			DetectorKey: finding.DetectorKey,
			Category:    finding.Category,
			Start:       finding.Start,
			End:         finding.End,
			ValueHash:   finding.ValueHash,
		})
	}
	return items
}
