package runtime

import (
	"context"

	"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/revisionstore"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
)

// loadRuntimePolicy hydrates policy bundles for the revision currently being assembled.
func loadRuntimePolicy(ctx context.Context, q revisionstore.Queryer, revisionID string, cfg *postgresRuntimeConfig) error {
	var err error
	cfg.PolicyBundles, err = policystore.LoadBundles(ctx, q, revisionID)
	return err
}
