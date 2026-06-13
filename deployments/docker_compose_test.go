package deployments

import (
	"os"
	"strings"
	"testing"
)

func TestDockerComposeInitializesClickHouseSchema(t *testing.T) {
	body, err := os.ReadFile("docker-compose.yml")
	if err != nil {
		t.Fatalf("ReadFile(docker-compose.yml) error = %v", err)
	}

	compose := string(body)
	for _, want := range []string{
		"../internal/contexts/traffic/adapters/clickhouse/schema.sql:/docker-entrypoint-initdb.d/001_kiwiguard_traffic_events.sql:ro",
		"CLICKHOUSE_DB: kiwiguard",
	} {
		if !strings.Contains(compose, want) {
			t.Fatalf("docker-compose.yml missing %q:\n%s", want, compose)
		}
	}
}
