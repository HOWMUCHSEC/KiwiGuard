package postgres

import (
	"context"
	"errors"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore"
	appcontrol "github.com/howmuchsec/kiwiguard/internal/contexts/control/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

// ConfigStatus compiles the active bundle set into the control-plane status response.
func (s postgresPolicyStore) ConfigStatus(ctx context.Context) (appcontrol.ConfigStatus, error) {
	cfg, err := s.repo.LoadActiveConfigSnapshot(ctx)
	if errors.Is(err, configstore.ErrActiveConfigNotFound) {
		return appcontrol.ConfigStatus{}, nil
	}
	if err != nil {
		return appcontrol.ConfigStatus{}, err
	}

	bundles := make([]policy.Bundle, 0, len(cfg.PolicyBundles))
	keys := make([]string, 0, len(cfg.PolicyBundles))
	routeNames := routeNamesByID(cfg.Routes)
	providerNames := providerNamesByID(cfg.Providers)
	for _, bundle := range cfg.PolicyBundles {
		if !bundle.Enabled {
			continue
		}
		dto := policyBundleFromPostgres(bundle, routeNames, providerNames)
		keys = append(keys, dto.Key)
		bundles = append(bundles, dto.ToPolicyBundle())
	}
	snapshot, err := policy.CompileSnapshot(bundles)
	if err != nil {
		return appcontrol.ConfigStatus{}, err
	}
	return appcontrol.ConfigStatus{
		ActivePolicyBundleKeys: keys,
		PolicySnapshotHash:     snapshot.Hash(),
	}, nil
}
