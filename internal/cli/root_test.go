package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRootCommandListsServiceModes(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := out.String()
	for _, want := range []string{"serve", "gateway", "control", "worker", "migrate"} {
		if !bytes.Contains([]byte(help), []byte(want)) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if got := out.String(); got != "kiwiguard dev\n" {
		t.Fatalf("version output = %q, want %q", got, "kiwiguard dev\n")
	}
}

func TestModesSupportDryRun(t *testing.T) {
	for _, mode := range []string{"serve", "gateway", "control", "worker"} {
		t.Run(mode, func(t *testing.T) {
			var out bytes.Buffer
			cmd := NewRootCommand()
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"--dry-run", mode})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
		})
	}
}

func TestModeCommandUsesCommandContext(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run", "control"})

	err := cmd.ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteContext() error = %v, want %v", err, context.Canceled)
	}
}

func TestDryRunFlagAfterMode(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"control", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestMigrateSupportsDryRun(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--dry-run", "migrate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestModeCommandReturnsConfigErrorWithoutDryRun(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"gateway"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want config error")
	}
	if !strings.Contains(err.Error(), "KIWIGUARD_POSTGRES_DSN is required") {
		t.Fatalf("Execute() error = %v, want missing postgres dsn", err)
	}
}

func TestMigrateCommandReturnsConfigErrorWithoutDryRun(t *testing.T) {
	t.Setenv("KIWIGUARD_POSTGRES_DSN", "")
	t.Setenv("KIWIGUARD_CLICKHOUSE_ADDR", "")

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"migrate"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want config error")
	}
	if !strings.Contains(err.Error(), "KIWIGUARD_POSTGRES_DSN is required") {
		t.Fatalf("Execute() error = %v, want missing postgres dsn", err)
	}
}
