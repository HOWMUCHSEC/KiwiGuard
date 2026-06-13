package httpapi

import "net/http"

// consoleSummaryResponse is the aggregated dashboard summary returned by the control API.
type consoleSummaryResponse struct {
	Version string                  `json:"version"`
	Config  consoleConfigSummary    `json:"config"`
	Policy  consolePolicySummary    `json:"policy"`
	Routing consoleRoutingSummary   `json:"routing"`
	Traffic trafficEventsSummaryDTO `json:"traffic"`
	Storage consoleStorageSummary   `json:"storage"`
}

type consoleConfigSummary struct {
	ActivePolicyBundleKeys []string `json:"active_policy_bundle_keys"`
	PolicySnapshotHash     string   `json:"policy_snapshot_hash"`
	Available              bool     `json:"available"`
}

type consolePolicySummary struct {
	ActiveBundleKeyCount int `json:"active_bundle_key_count"`
	BundleCount          int `json:"bundle_count"`
}

type consoleRoutingSummary struct {
	ModelMappingCount    int `json:"model_mapping_count"`
	VerdictProviderCount int `json:"verdict_provider_count"`
}

type consoleStorageSummary struct {
	Available        bool    `json:"available"`
	Enabled          bool    `json:"enabled"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
	Depth            int     `json:"depth"`
	Bytes            int64   `json:"bytes"`
	MaxBytes         int64   `json:"max_bytes"`
	OldestAgeSeconds float64 `json:"oldest_age_seconds"`
	OverflowCount    uint64  `json:"overflow_count"`
}

// consoleSummaryHandler aggregates summary data from config, traffic, and spool dependencies.
func consoleSummaryHandler(version string, store PolicyStore, spool SpoolStatusProvider, traffic TrafficReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary := consoleSummaryResponse{Version: version}

		if store != nil {
			status, err := store.ConfigStatus(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console_summary_failed", "config status query failed")
				return
			}
			activeKeys := append([]string{}, status.ActivePolicyBundleKeys...)
			summary.Config = consoleConfigSummary{
				ActivePolicyBundleKeys: activeKeys,
				PolicySnapshotHash:     status.PolicySnapshotHash,
				Available:              true,
			}
			summary.Policy.ActiveBundleKeyCount = len(status.ActivePolicyBundleKeys)

			bundles, err := store.ListPolicyBundles(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console_summary_failed", "policy bundle query failed")
				return
			}
			summary.Policy.BundleCount = len(bundles)

			mappings, err := store.ListModelMappings(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console_summary_failed", "model mapping query failed")
				return
			}
			summary.Routing.ModelMappingCount = len(mappings)

			providers, err := store.ListVerdictProviders(r.Context())
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console_summary_failed", "verdict provider query failed")
				return
			}
			summary.Routing.VerdictProviderCount = len(providers)
		}

		if traffic != nil {
			trafficSummary, err := traffic.SummarizeTrafficEvents(r.Context(), trafficEventFilter{})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console_summary_failed", "traffic summary query failed")
				return
			}
			summary.Traffic = trafficSummary
		}

		if spool != nil {
			status := spool.SpoolStatus()
			summary.Storage = consoleStorageSummary{
				Available:        true,
				Enabled:          status.Enabled,
				Status:           status.Status,
				Reason:           status.Reason,
				Depth:            status.Depth,
				Bytes:            status.Bytes,
				MaxBytes:         status.MaxBytes,
				OldestAgeSeconds: status.OldestAgeSeconds,
				OverflowCount:    status.OverflowCount,
			}
		}

		writeJSON(w, http.StatusOK, summary)
	}
}
