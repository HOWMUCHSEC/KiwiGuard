package httpapi

import (
	"net/http"
)

// SpoolStatus reports live durable event spool capacity and replay backlog.
type SpoolStatus struct {
	Enabled          bool    `json:"enabled"`
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
	Depth            int     `json:"depth"`
	Bytes            int64   `json:"bytes"`
	MaxBytes         int64   `json:"max_bytes"`
	OldestAgeSeconds float64 `json:"oldest_age_seconds"`
	OverflowCount    uint64  `json:"overflow_count"`
}

// SpoolStatusProvider reports live durable event spool state.
type SpoolStatusProvider interface {
	SpoolStatus() SpoolStatus
}

func trafficSpoolHandler(provider SpoolStatusProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if provider == nil {
			writeError(w, http.StatusServiceUnavailable, "spool_status_unavailable", "spool status is unavailable")
			return
		}

		writeJSON(w, http.StatusOK, provider.SpoolStatus())
	}
}
