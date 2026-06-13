package bootstrap

import (
	"context"
	"net/http"

	controlassembly "github.com/howmuchsec/kiwiguard/internal/bootstrap/control"
	controlhttp "github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"
)

// ControlHandler builds the control API handler.
func (f *Factory) ControlHandler(ctx context.Context, version string) (http.Handler, Cleanup, error) {
	if err := f.validateRequiredDependencies(); err != nil {
		return nil, nil, err
	}
	if f.options.Store != nil || f.options.Repository != nil {
		return f.controlHandler(controlassembly.BuildHandler(controlhttp.ServerOptions{
			Version:        version,
			Store:          f.options.Store,
			Notifier:       f.options.Notifier,
			ConfigHealth:   f.configHealth,
			MetricsHandler: f.metrics.Handler(),
			HTTPMiddleware: f.httpMiddleware("control"),
			BearerToken:    f.options.Config.ControlAuthToken,
		})), noopCleanup, nil
	}

	deps, cleanup, err := f.productionDeps(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	return f.controlHandler(controlassembly.BuildHandler(controlhttp.ServerOptions{
		Version:        version,
		Store:          deps.controlStore,
		Notifier:       deps.notifier,
		ConfigHealth:   f.configHealth,
		AuditHealth:    deps.eventGate,
		SpoolStatus:    deps.spoolStatus,
		TrafficReader:  controlhttp.NewClickHouseTrafficReader(deps.clickhouseConn),
		MetricsHandler: f.metrics.Handler(),
		HTTPMiddleware: f.httpMiddleware("control"),
		BearerToken:    f.options.Config.ControlAuthToken,
	})), cleanup, nil
}
