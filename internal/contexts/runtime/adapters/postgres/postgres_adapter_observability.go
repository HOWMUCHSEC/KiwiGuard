package runtime

import (
	"encoding/json"

	observabilitystore "github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters/postgres/observability"
)

// convertPostgresSinks converts persisted sink records into runtime sink configuration.
func convertPostgresSinks(input []observabilitystore.Sink) []SinkConfig {
	sinks := make([]SinkConfig, 0, len(input))
	for _, sink := range input {
		if !sink.Enabled {
			continue
		}
		sinks = append(sinks, SinkConfig{
			ID:     sink.ID,
			Key:    sink.Name,
			Kind:   sink.Kind,
			Config: append(json.RawMessage(nil), sink.Config...),
		})
	}
	return sinks
}

// convertPostgresRetentionPolicies converts persisted retention policies into runtime configuration.
func convertPostgresRetentionPolicies(policies []observabilitystore.RetentionPolicy, sinkNames map[string]string) []RetentionPolicyConfig {
	retention := make([]RetentionPolicyConfig, 0, len(policies))
	for _, policy := range policies {
		retention = append(retention, RetentionPolicyConfig{
			ID:            policy.ID,
			Key:           policy.Name,
			SinkKey:       sinkNames[policy.SinkID],
			EventType:     policy.EventType,
			RetentionDays: policy.RetentionDays,
		})
	}
	return retention
}

// convertPostgresRawCapturePolicies converts persisted raw-capture records into runtime configuration.
func convertPostgresRawCapturePolicies(policies []observabilitystore.RawCapturePolicy, routeNames map[string]string) []RawCaptureConfig {
	rawCapture := make([]RawCaptureConfig, 0, len(policies))
	for _, capture := range policies {
		rawCapture = append(rawCapture, RawCaptureConfig{
			ID:            capture.ID,
			RouteKey:      routeNames[capture.RouteID],
			Direction:     capture.Direction,
			Enabled:       capture.Enabled,
			SampleRate:    capture.SampleRate,
			RedactionMode: capture.RedactionMode,
		})
	}
	return rawCapture
}
