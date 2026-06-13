package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

type healthResponse struct {
	Status    string                 `json:"status"`
	Version   string                 `json:"version"`
	Timestamp string                 `json:"timestamp"`
	Checks    map[string]healthCheck `json:"checks,omitempty"`
}

type healthCheck struct {
	Status           string  `json:"status"`
	Reason           string  `json:"reason,omitempty"`
	Depth            int     `json:"depth,omitempty"`
	Bytes            int64   `json:"bytes,omitempty"`
	MaxBytes         int64   `json:"max_bytes,omitempty"`
	OldestAgeSeconds float64 `json:"oldest_age_seconds,omitempty"`
	OverflowCount    uint64  `json:"overflow_count,omitempty"`
}

// ConfigHealth reports active config readiness.
type ConfigHealth interface {
	ConfigReady() bool
	Reason() string
}

// AuditHealth reports audit sink readiness.
type AuditHealth interface {
	Healthy() bool
	Reason() string
}

func healthHandler(version string, configHealth ConfigHealth, auditHealth AuditHealth, spoolStatus SpoolStatusProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		code := http.StatusOK
		checks := make(map[string]healthCheck)
		if configHealth != nil {
			check := healthCheck{Status: "ok"}
			if !configHealth.ConfigReady() {
				check.Status = "degraded"
				check.Reason = configHealth.Reason()
				status = "degraded"
				code = http.StatusServiceUnavailable
			}
			checks["config"] = check
		}
		if auditHealth != nil {
			check := healthCheck{Status: "ok"}
			if !auditHealth.Healthy() {
				check.Status = "unhealthy"
				check.Reason = auditHealth.Reason()
				status = "unhealthy"
				code = http.StatusServiceUnavailable
			}
			checks["audit_sink"] = check
		}
		if spoolStatus != nil {
			spool := spoolStatus.SpoolStatus()
			check := healthCheck{
				Status:           spool.Status,
				Reason:           spool.Reason,
				Depth:            spool.Depth,
				Bytes:            spool.Bytes,
				MaxBytes:         spool.MaxBytes,
				OldestAgeSeconds: spool.OldestAgeSeconds,
				OverflowCount:    spool.OverflowCount,
			}
			if check.Status == "" {
				check.Status = "unknown"
			}
			if check.Status != "ok" && check.Status != "disabled" {
				if status == "ok" {
					status = "degraded"
				}
				if code == http.StatusOK {
					code = http.StatusServiceUnavailable
				}
			}
			checks["event_spool"] = check
		}
		if len(checks) == 0 {
			checks = nil
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		if err := json.NewEncoder(w).Encode(healthResponse{
			Status:    status,
			Version:   version,
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Checks:    checks,
		}); err != nil {
			return
		}
	}
}
