package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/bootstrap"
	"github.com/howmuchsec/kiwiguard/internal/config"
)

func TestNewRunnerSupportsKnownModes(t *testing.T) {
	for _, mode := range []Mode{ModeServe, ModeGateway, ModeControl, ModeWorker} {
		runner := NewRunner(RunnerOptions{Mode: mode, Version: "test"})
		if runner.Mode() != mode {
			t.Fatalf("Mode() = %q, want %q", runner.Mode(), mode)
		}
	}
}

func TestRunnerDryRun(t *testing.T) {
	runner := NewRunner(RunnerOptions{Mode: ModeControl, Version: "test", DryRun: true})
	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunnerDefaultRuntimeFactoryUsesBootstrapCompositionRoot(t *testing.T) {
	runner := NewRunner(RunnerOptions{Config: config.Config{}})

	if _, ok := runner.runtimeFactory().(*bootstrap.Factory); !ok {
		t.Fatalf("runtimeFactory() type = %T, want bootstrap Factory", runner.runtimeFactory())
	}
}

func TestRunnerStartsGatewayMode(t *testing.T) {
	var started []Mode
	cleaned := false
	runner := NewRunner(RunnerOptions{
		Mode:    ModeGateway,
		Version: "test",
		RuntimeFactory: fakeRuntimeFactory{
			cleanup: func(ctx context.Context) error {
				cleaned = true
				return nil
			},
		},
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			started = append(started, mode)
			return nil
		}),
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: ":0",
		},
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(started) != 1 || started[0] != ModeGateway {
		t.Fatalf("started = %+v, want gateway", started)
	}
	if !cleaned {
		t.Fatal("runtime cleanup was not called")
	}
}

func TestRunnerStartsControlMode(t *testing.T) {
	var started []Mode
	var gotAddr string
	runner := NewRunner(RunnerOptions{
		Mode:           ModeControl,
		Version:        "test",
		RuntimeFactory: fakeRuntimeFactory{},
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			started = append(started, mode)
			gotAddr = addr
			return nil
		}),
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: "127.0.0.1:0",
		},
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(started) != 1 || started[0] != ModeControl {
		t.Fatalf("started = %+v, want control", started)
	}
	if gotAddr != "127.0.0.1:0" {
		t.Fatalf("addr = %q, want %q", gotAddr, "127.0.0.1:0")
	}
}

func TestRunnerStartsServeMode(t *testing.T) {
	var started []Mode
	var mu sync.Mutex
	ctx, cancel := context.WithCancel(context.Background())
	runner := NewRunner(RunnerOptions{
		Mode:    ModeServe,
		Version: "test",
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			mu.Lock()
			started = append(started, mode)
			if len(started) == 3 {
				cancel()
			}
			mu.Unlock()
			<-ctx.Done()
			return nil
		}),
		RuntimeFactory: fakeRuntimeFactory{
			worker: WorkerRunnerFunc(func(ctx context.Context) error {
				mu.Lock()
				started = append(started, ModeWorker)
				if len(started) == 3 {
					cancel()
				}
				mu.Unlock()
				<-ctx.Done()
				return nil
			}),
		},
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: ":0",
		},
	})

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(started) != 3 || !startedMode(started, ModeGateway) || !startedMode(started, ModeControl) || !startedMode(started, ModeWorker) {
		t.Fatalf("started = %+v, want gateway, control, and worker", started)
	}
}

func TestRunnerServeModeCancelsOtherServerAfterError(t *testing.T) {
	wantErr := errors.New("gateway failed")
	runner := NewRunner(RunnerOptions{
		Mode:           ModeServe,
		Version:        "test",
		RuntimeFactory: fakeRuntimeFactory{},
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			if mode == ModeGateway {
				return wantErr
			}
			<-ctx.Done()
			return nil
		}),
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: ":0",
		},
	})

	err := runner.Run(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRunnerServeModeConvertsChildPanicToError(t *testing.T) {
	runner := NewRunner(RunnerOptions{
		Mode:           ModeServe,
		Version:        "test",
		RuntimeFactory: fakeRuntimeFactory{},
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			if mode == ModeGateway {
				panic("gateway boom")
			}
			<-ctx.Done()
			return nil
		}),
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: ":0",
		},
	})

	err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want panic converted to error")
	}
	if got := err.Error(); !strings.Contains(got, "gateway panicked: gateway boom") || !strings.Contains(got, "goroutine") {
		t.Fatalf("Run() error = %q, want panic value and stack", got)
	}
}

func TestRunnerReturnsFactoryErrors(t *testing.T) {
	wantErr := errors.New("factory failed")
	tests := []struct {
		name string
		mode Mode
	}{
		{name: "gateway", mode: ModeGateway},
		{name: "control", mode: ModeControl},
		{name: "worker", mode: ModeWorker},
		{name: "serve", mode: ModeServe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(RunnerOptions{
				Mode:           tt.mode,
				RuntimeFactory: fakeRuntimeFactory{err: wantErr},
				ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
					t.Fatalf("ServerStarter called after factory error")
					return nil
				}),
			})

			err := runner.Run(context.Background())
			if !errors.Is(err, wantErr) {
				t.Fatalf("Run() error = %v, want %v", err, wantErr)
			}
		})
	}
}

func TestRunnerStartsWorkerMode(t *testing.T) {
	started := false
	runner := NewRunner(RunnerOptions{
		Mode: ModeWorker,
		RuntimeFactory: fakeRuntimeFactory{
			worker: WorkerRunnerFunc(func(ctx context.Context) error {
				started = true
				return nil
			}),
		},
		Config: config.Config{
			GatewayAddr: ":0",
			ControlAddr: ":0",
		},
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			t.Fatalf("ServerStarter called for worker mode")
			return nil
		}),
	})

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !started {
		t.Fatal("worker was not started")
	}
}

func TestRunnerUnknownModeReturnsError(t *testing.T) {
	runner := NewRunner(RunnerOptions{
		Mode: "mystery",
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			t.Fatalf("ServerStarter called for unknown mode")
			return nil
		}),
	})

	err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want unknown mode error")
	}
	if got, want := err.Error(), `unknown mode "mystery"`; got != want {
		t.Fatalf("Run() error = %q, want %q", got, want)
	}
}

func TestRunnerReturnsContextErrorBeforeStarting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := NewRunner(RunnerOptions{
		Mode: ModeGateway,
		ServerStarter: ServerStarterFunc(func(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
			t.Fatalf("ServerStarter called for canceled context")
			return nil
		}),
	})

	err := runner.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want %v", err, context.Canceled)
	}
}

func TestHTTPServerStarterReturnsListenError(t *testing.T) {
	err := HTTPServerStarter{}.StartServer(context.Background(), ModeControl, http.NewServeMux(), "127.0.0.1:not-a-port")
	if err == nil {
		t.Fatal("StartServer() error = nil, want listen error")
	}
}

func TestHTTPServerStarterBuildsServerWithTimeouts(t *testing.T) {
	starter := HTTPServerStarter{Options: HTTPServerOptions{
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       3 * time.Second,
		WriteTimeout:      4 * time.Second,
		IdleTimeout:       5 * time.Second,
	}}

	server := starter.newServer(context.Background(), http.NewServeMux(), "127.0.0.1:0")

	if server.ReadHeaderTimeout != 2*time.Second {
		t.Fatalf("ReadHeaderTimeout = %v, want 2s", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != 3*time.Second {
		t.Fatalf("ReadTimeout = %v, want 3s", server.ReadTimeout)
	}
	if server.WriteTimeout != 4*time.Second {
		t.Fatalf("WriteTimeout = %v, want 4s", server.WriteTimeout)
	}
	if server.IdleTimeout != 5*time.Second {
		t.Fatalf("IdleTimeout = %v, want 5s", server.IdleTimeout)
	}
}

func TestHTTPServerStarterShutdownContextUsesTimeout(t *testing.T) {
	starter := HTTPServerStarter{Options: HTTPServerOptions{ShutdownTimeout: 25 * time.Millisecond}}

	ctx, cancel := starter.shutdownContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("shutdown context has no deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Second {
		t.Fatalf("shutdown deadline remaining = %v, want positive timeout", remaining)
	}
}

func TestHTTPServerStarterShutsDownWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := HTTPServerStarter{}.StartServer(ctx, ModeControl, http.NewServeMux(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("StartServer() error = %v", err)
	}
}

func TestHTTPServerOptionsFromConfig(t *testing.T) {
	cfg := config.Config{
		ServerReadHeaderTimeout: 2 * time.Second,
		ServerReadTimeout:       3 * time.Second,
		ServerWriteTimeout:      4 * time.Second,
		ServerIdleTimeout:       5 * time.Second,
		ShutdownTimeout:         6 * time.Second,
	}

	opts := httpServerOptionsFromConfig(cfg)
	if opts.ReadHeaderTimeout != cfg.ServerReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", opts.ReadHeaderTimeout, cfg.ServerReadHeaderTimeout)
	}
	if opts.ReadTimeout != cfg.ServerReadTimeout {
		t.Fatalf("ReadTimeout = %v, want %v", opts.ReadTimeout, cfg.ServerReadTimeout)
	}
	if opts.WriteTimeout != cfg.ServerWriteTimeout {
		t.Fatalf("WriteTimeout = %v, want %v", opts.WriteTimeout, cfg.ServerWriteTimeout)
	}
	if opts.IdleTimeout != cfg.ServerIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", opts.IdleTimeout, cfg.ServerIdleTimeout)
	}
	if opts.ShutdownTimeout != cfg.ShutdownTimeout {
		t.Fatalf("ShutdownTimeout = %v, want %v", opts.ShutdownTimeout, cfg.ShutdownTimeout)
	}
}

func TestHTTPServerStarterOptionsWithDefaults(t *testing.T) {
	opts := HTTPServerStarter{}.optionsWithDefaults()
	if opts.ReadHeaderTimeout != defaultServerReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v, want default", opts.ReadHeaderTimeout)
	}
	if opts.ReadTimeout != defaultServerReadTimeout {
		t.Fatalf("ReadTimeout = %v, want default", opts.ReadTimeout)
	}
	if opts.WriteTimeout != defaultServerWriteTimeout {
		t.Fatalf("WriteTimeout = %v, want default", opts.WriteTimeout)
	}
	if opts.IdleTimeout != defaultServerIdleTimeout {
		t.Fatalf("IdleTimeout = %v, want default", opts.IdleTimeout)
	}
	if opts.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("ShutdownTimeout = %v, want default", opts.ShutdownTimeout)
	}
}

func TestChildErrorIgnoresCancellationAfterContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := childError(ctx, ModeGateway, context.Canceled); err != nil {
		t.Fatalf("childError(canceled) = %v, want nil", err)
	}
	if err := childError(ctx, ModeGateway, nil); err != nil {
		t.Fatalf("childError(nil after context done) = %v, want nil", err)
	}
}

func TestChildErrorReportsUnexpectedExitAndNonCancellationErrors(t *testing.T) {
	if err := childError(context.Background(), ModeWorker, nil); err == nil || err.Error() != "worker exited unexpectedly" {
		t.Fatalf("childError(nil) = %v, want unexpected worker exit", err)
	}

	wantErr := errors.New("server failed")
	if err := childError(context.Background(), ModeControl, wantErr); !errors.Is(err, wantErr) {
		t.Fatalf("childError(error) = %v, want %v", err, wantErr)
	}
}

func TestRecoverPanicAsError(t *testing.T) {
	err := recoverPanicAsError("worker", func() error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("recoverPanicAsError() error = nil, want panic error")
	}
	if got := err.Error(); !strings.Contains(got, "worker panicked: boom") || !strings.Contains(got, "goroutine") {
		t.Fatalf("recoverPanicAsError() error = %q, want panic value and stack", got)
	}
}

func TestRunCleanupAllowsNilCleanup(t *testing.T) {
	runCleanup(context.Background(), nil)
}

func startedMode(started []Mode, want Mode) bool {
	for _, mode := range started {
		if mode == want {
			return true
		}
	}
	return false
}

type fakeRuntimeFactory struct {
	worker  WorkerRunner
	cleanup RuntimeCleanup
	err     error
}

func (f fakeRuntimeFactory) GatewayHandler(ctx context.Context) (http.Handler, RuntimeCleanup, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return http.NewServeMux(), f.runtimeCleanup(), nil
}

func (f fakeRuntimeFactory) ControlHandler(ctx context.Context, version string) (http.Handler, RuntimeCleanup, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return http.NewServeMux(), f.runtimeCleanup(), nil
}

func (f fakeRuntimeFactory) Worker(ctx context.Context) (WorkerRunner, RuntimeCleanup, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.worker == nil {
		return WorkerRunnerFunc(func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		}), f.runtimeCleanup(), nil
	}
	return f.worker, f.runtimeCleanup(), nil
}

func (f fakeRuntimeFactory) runtimeCleanup() RuntimeCleanup {
	if f.cleanup != nil {
		return f.cleanup
	}
	return noopRuntimeCleanup
}

func noopRuntimeCleanup(ctx context.Context) error {
	return nil
}

// WorkerRunnerFunc adapts a function into a WorkerRunner.
type WorkerRunnerFunc func(context.Context) error

// Run runs f.
func (f WorkerRunnerFunc) Run(ctx context.Context) error {
	return f(ctx)
}
