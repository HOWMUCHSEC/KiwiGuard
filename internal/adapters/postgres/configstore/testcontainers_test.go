package configstore

import (
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

func skipIfTestcontainersUnavailable(t *testing.T) {
	t.Helper()

	testcontainers.SkipIfProviderIsNotHealthy(t)
}
