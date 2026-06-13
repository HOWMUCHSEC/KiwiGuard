package bootstrap

import (
	"testing"

	"github.com/howmuchsec/kiwiguard/internal/config"
)

func TestNewFactoryCreatesProductionFactory(t *testing.T) {
	factory := NewFactory(config.Config{})

	if factory == nil {
		t.Fatal("NewFactory() = nil, want factory")
	}
	if factory.configHealth == nil {
		t.Fatal("factory config health = nil, want readiness state")
	}
}
