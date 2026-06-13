package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/howmuchsec/kiwiguard/internal/cli"
)

const panicExitCode = 2

var newRootCommand = cli.NewRootCommand

// main delegates process execution to runMain so tests can exercise CLI lifecycle behavior.
func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

// runMain executes the CLI with signal-aware shutdown and top-level panic recovery.
func runMain(args []string, stdout, stderr io.Writer) (code int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			_, _ = fmt.Fprintf(stderr, "kiwiguard panic: %v\n%s", recovered, debug.Stack())
			code = panicExitCode
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, args, stdout, stderr)
}

// run configures the root command and returns a process-style exit code.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cmd := newRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
