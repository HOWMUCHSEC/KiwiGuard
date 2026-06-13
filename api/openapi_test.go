package api_test

import (
	"os"
	"strings"
	"testing"
)

func TestOpenAPIContractDocumentsControlHealth(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatalf("read openapi.yaml: %v", err)
	}

	spec := string(contents)
	for _, want := range []string{
		"openapi: 3.1.0",
		"/api/healthz:",
		"HealthResponse:",
		"/api/config/active:",
		"/api/policy-bundles:",
		"/api/policy-bundles/validate:",
		"/api/policy-bundles/activate:",
		"/api/model-mappings:",
		"/api/model-mappings/{id}:",
		"/api/verdict-providers:",
		"/api/verdict-providers/{id}:",
		"/api/tools/regex-test:",
		"/api/tools/policy-dry-run:",
		"/api/traffic/events:",
		"/api/traffic/spool:",
		"ConfigStatusResponse:",
		"PolicyBundle:",
		"TrafficEvent:",
		"SpoolStatusResponse:",
		"request_payload:",
		"response_payload:",
		"spool_status:",
		"oldest_age_seconds:",
		"overflow_count:",
		"RegexTestResponse:",
		"PolicyDryRunResponse:",
		"ErrorResponse:",
		"status:",
		"version:",
		"timestamp:",
	} {
		if !strings.Contains(spec, want) {
			t.Errorf("openapi.yaml missing %q", want)
		}
	}
}
