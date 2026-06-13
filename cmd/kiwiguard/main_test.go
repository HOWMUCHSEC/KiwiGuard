package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunReturnsFailureForInvalidCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{"bogus"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("run() code = 0, want failure")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "bogus"`) {
		t.Fatalf("stderr = %q, want unknown command error", stderr.String())
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() code = %d, want 0", code)
	}
	if got := stdout.String(); got != "kiwiguard dev\n" {
		t.Fatalf("stdout = %q, want version", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunReturnsFailureForCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(ctx, []string{"--dry-run", "control"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), context.Canceled.Error()) {
		t.Fatalf("stderr = %q, want canceled context", stderr.String())
	}
}

func TestRunMainRecoversPanic(t *testing.T) {
	original := newRootCommand
	t.Cleanup(func() {
		newRootCommand = original
	})
	newRootCommand = func() *cobra.Command {
		return &cobra.Command{
			Use: "kiwiguard",
			Run: func(cmd *cobra.Command, args []string) {
				panic("boom")
			},
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runMain(nil, &stdout, &stderr)
	if code != panicExitCode {
		t.Fatalf("runMain() code = %d, want %d", code, panicExitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "kiwiguard panic: boom") || !strings.Contains(got, "goroutine") {
		t.Fatalf("stderr = %q, want panic value and stack", got)
	}
}
