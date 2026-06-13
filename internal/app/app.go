package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/howmuchsec/kiwiguard/internal/bootstrap"
	"github.com/howmuchsec/kiwiguard/internal/config"
)

// Mode identifies a KiwiGuard runtime mode.
type Mode string

const (
	// ModeServe runs all KiwiGuard services together.
	ModeServe Mode = "serve"
	// ModeGateway runs the data plane.
	ModeGateway Mode = "gateway"
	// ModeControl runs the control plane.
	ModeControl Mode = "control"
	// ModeWorker runs background worker tasks.
	ModeWorker Mode = "worker"
)

// RunnerOptions selects the runtime mode and injected dependencies for one process.
type RunnerOptions struct {
	Mode           Mode
	Version        string
	DryRun         bool
	Config         config.Config
	ServerStarter  ServerStarter
	RuntimeFactory RuntimeFactory
}

// ServerStarter starts an HTTP server for a runtime mode.
type ServerStarter interface {
	StartServer(ctx context.Context, mode Mode, handler http.Handler, addr string) error
}

// RuntimeCleanup releases resources allocated for a runtime child.
type RuntimeCleanup = func(context.Context) error

// WorkerRunner runs background worker tasks.
type WorkerRunner = interface {
	Run(context.Context) error
}

// RuntimeFactory builds production runtime children.
type RuntimeFactory interface {
	GatewayHandler(context.Context) (http.Handler, RuntimeCleanup, error)
	ControlHandler(context.Context, string) (http.Handler, RuntimeCleanup, error)
	Worker(context.Context) (WorkerRunner, RuntimeCleanup, error)
}

// ServerStarterFunc adapts a function into a ServerStarter.
type ServerStarterFunc func(ctx context.Context, mode Mode, handler http.Handler, addr string) error

// StartServer starts an HTTP server.
func (f ServerStarterFunc) StartServer(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
	return f(ctx, mode, handler, addr)
}

// Runner executes a KiwiGuard runtime mode.
type Runner struct {
	options RunnerOptions
}

// NewRunner wires process-level runtime orchestration from the supplied options.
func NewRunner(options RunnerOptions) *Runner {
	return &Runner{options: options}
}

// Mode reports which KiwiGuard runtime mode the runner will execute.
func (r *Runner) Mode() Mode {
	return r.options.Mode
}

// Run starts the configured runtime mode.
func (r *Runner) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.options.DryRun {
		return nil
	}

	starter := r.options.ServerStarter
	if starter == nil {
		starter = HTTPServerStarter{Options: httpServerOptionsFromConfig(r.options.Config)}
	}

	switch r.options.Mode {
	case ModeGateway:
		factory := r.runtimeFactory()
		handler, cleanup, err := factory.GatewayHandler(ctx)
		if err != nil {
			return err
		}
		defer runCleanup(ctx, cleanup)
		return starter.StartServer(ctx, ModeGateway, handler, r.options.Config.GatewayAddr)
	case ModeControl:
		factory := r.runtimeFactory()
		handler, cleanup, err := factory.ControlHandler(ctx, r.options.Version)
		if err != nil {
			return err
		}
		defer runCleanup(ctx, cleanup)
		return starter.StartServer(ctx, ModeControl, handler, r.options.Config.ControlAddr)
	case ModeServe:
		return r.runServe(ctx, starter)
	case ModeWorker:
		factory := r.runtimeFactory()
		worker, cleanup, err := factory.Worker(ctx)
		if err != nil {
			return err
		}
		defer runCleanup(ctx, cleanup)
		return worker.Run(ctx)
	default:
		return fmt.Errorf("unknown mode %q", r.options.Mode)
	}
}

func (r *Runner) runServe(ctx context.Context, starter ServerStarter) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	factory := r.runtimeFactory()
	gatewayHandler, gatewayCleanup, err := factory.GatewayHandler(ctx)
	if err != nil {
		return err
	}
	defer runCleanup(ctx, gatewayCleanup)
	controlHandler, controlCleanup, err := factory.ControlHandler(ctx, r.options.Version)
	if err != nil {
		return err
	}
	defer runCleanup(ctx, controlCleanup)
	worker, workerCleanup, err := factory.Worker(ctx)
	if err != nil {
		return err
	}
	defer runCleanup(ctx, workerCleanup)

	errCh := make(chan error, 3)
	startRuntimeChild(ctx, errCh, ModeGateway, func() error {
		return starter.StartServer(ctx, ModeGateway, gatewayHandler, r.options.Config.GatewayAddr)
	})
	startRuntimeChild(ctx, errCh, ModeControl, func() error {
		return starter.StartServer(ctx, ModeControl, controlHandler, r.options.Config.ControlAddr)
	})
	startRuntimeChild(ctx, errCh, ModeWorker, func() error {
		return worker.Run(ctx)
	})

	var firstErr error
	for range 3 {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}
	return firstErr
}

func startRuntimeChild(ctx context.Context, errCh chan<- error, mode Mode, run func() error) {
	go func() {
		errCh <- childError(ctx, mode, recoverPanicAsError(string(mode), run))
	}()
}

func (r *Runner) runtimeFactory() RuntimeFactory {
	if r.options.RuntimeFactory != nil {
		return r.options.RuntimeFactory
	}
	return bootstrap.NewFactory(r.options.Config)
}

func childError(ctx context.Context, mode Mode, err error) error {
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return nil
		}
		return err
	}
	if ctx.Err() != nil {
		return nil
	}
	return fmt.Errorf("%s exited unexpectedly", mode)
}

func recoverPanicAsError(scope string, run func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panicked: %v\n%s", scope, recovered, debug.Stack())
		}
	}()
	return run()
}

func runCleanup(ctx context.Context, cleanup RuntimeCleanup) {
	if cleanup == nil {
		return
	}
	_ = cleanup(ctx)
}

const (
	defaultServerReadHeaderTimeout = 5 * time.Second
	defaultServerReadTimeout       = 15 * time.Second
	defaultServerWriteTimeout      = time.Minute
	defaultServerIdleTimeout       = 2 * time.Minute
	defaultShutdownTimeout         = 10 * time.Second
)

// HTTPServerOptions defines listener and shutdown timeouts for managed HTTP servers.
type HTTPServerOptions struct {
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// HTTPServerStarter starts HTTP servers and shuts them down when the context is canceled.
type HTTPServerStarter struct {
	Options HTTPServerOptions
}

// StartServer starts an HTTP server for a runtime mode.
func (s HTTPServerStarter) StartServer(ctx context.Context, mode Mode, handler http.Handler, addr string) error {
	if addr == "" {
		addr = ":0"
	}

	server := s.newServer(ctx, handler, addr)

	errCh := make(chan error, 1)
	go func() {
		errCh <- recoverPanicAsError(string(mode)+" server", func() error {
			err := server.ListenAndServe()
			if err == http.ErrServerClosed {
				err = nil
			}
			return err
		})
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := s.shutdownContext()
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func (s HTTPServerStarter) newServer(ctx context.Context, handler http.Handler, addr string) *http.Server {
	opts := s.optionsWithDefaults()
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: opts.ReadHeaderTimeout,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
		BaseContext: func(listener net.Listener) context.Context {
			return ctx
		},
	}
}

func (s HTTPServerStarter) shutdownContext() (context.Context, context.CancelFunc) {
	timeout := s.optionsWithDefaults().ShutdownTimeout
	return context.WithTimeout(context.Background(), timeout)
}

func (s HTTPServerStarter) optionsWithDefaults() HTTPServerOptions {
	opts := s.Options
	if opts.ReadHeaderTimeout <= 0 {
		opts.ReadHeaderTimeout = defaultServerReadHeaderTimeout
	}
	if opts.ReadTimeout <= 0 {
		opts.ReadTimeout = defaultServerReadTimeout
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = defaultServerWriteTimeout
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = defaultServerIdleTimeout
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = defaultShutdownTimeout
	}
	return opts
}

func httpServerOptionsFromConfig(cfg config.Config) HTTPServerOptions {
	httpCfg := cfg.HTTPServer()
	return HTTPServerOptions{
		ReadHeaderTimeout: httpCfg.ReadHeaderTimeout,
		ReadTimeout:       httpCfg.ReadTimeout,
		WriteTimeout:      httpCfg.WriteTimeout,
		IdleTimeout:       httpCfg.IdleTimeout,
		ShutdownTimeout:   httpCfg.ShutdownTimeout,
	}
}
