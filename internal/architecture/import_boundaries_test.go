package architecture

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeDoesNotImportRetiredGatewayPackage(t *testing.T) {
	root := repositoryRoot(t)
	runtimeDir := filepath.Join(root, "internal", "contexts", "runtime")
	forbidden := retiredGatewayImport()

	var offenders []string
	err := filepath.WalkDir(runtimeDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), forbidden) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("runtime package must not import retired gateway package: %s", strings.Join(offenders, ", "))
	}
}

func TestRetiredGatewayPackageDoesNotReappear(t *testing.T) {
	root := repositoryRoot(t)
	retiredDir := filepath.Join(root, "internal", "gateway")
	if _, err := os.Stat(retiredDir); !os.IsNotExist(err) {
		t.Fatalf("retired gateway package directory must not exist: %s", retiredDir)
	}

	forbidden := retiredGatewayImport()
	var offenders []string
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(body), forbidden) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("retired gateway package import must not reappear: %s", strings.Join(offenders, ", "))
	}
}

func TestRetiredInfrastructurePackagesDoNotReappear(t *testing.T) {
	root := repositoryRoot(t)
	retiredDirs := []string{
		filepath.Join(root, "internal", "application", "control"),
		filepath.Join(root, "internal", "application", "runtime"),
		filepath.Join(root, "internal", "adapters", "http", "control"),
		filepath.Join(root, "internal", "adapters", "http", "verdict"),
		filepath.Join(root, "internal", "adapters", "clickhouse"),
		filepath.Join(root, "internal", "adapters", "postgres", "control"),
		filepath.Join(root, "internal", "adapters", "traffic"),
		filepath.Join(root, "internal", "control"),
		filepath.Join(root, "internal", "domain", "clients"),
		filepath.Join(root, "internal", "domain", "detection"),
		filepath.Join(root, "internal", "domain", "policy"),
		filepath.Join(root, "internal", "domain", "routing"),
		filepath.Join(root, "internal", "domain", "traffic"),
		filepath.Join(root, "internal", "domain", "verdict"),
		filepath.Join(root, "internal", "adapters", "postgres", "runtime"),
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "runtime"),
		filepath.Join(root, "internal", "storage"),
		filepath.Join(root, "internal", "detectors"),
		filepath.Join(root, "internal", "events"),
		filepath.Join(root, "internal", "policy"),
		filepath.Join(root, "internal", "runtime"),
		filepath.Join(root, "internal", "verdict"),
	}
	for _, retiredDir := range retiredDirs {
		if _, err := os.Stat(retiredDir); !os.IsNotExist(err) {
			t.Fatalf("retired infrastructure or compatibility package directory must not exist: %s", retiredDir)
		}
	}
}

func TestDomainAndApplicationDoNotImportInfrastructure(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		retiredGatewayImport(),
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/storage/`,
		`"github.com/howmuchsec/kiwiguard/internal/detectors"`,
		`"github.com/howmuchsec/kiwiguard/internal/control"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/clients"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/detection"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/policy"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/routing"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/traffic"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/clickhouse"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/traffic/events"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/http/verdict"`,
		`"github.com/howmuchsec/kiwiguard/internal/domain/verdict"`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/control/adapters/httpapi"`,
		`"github.com/howmuchsec/kiwiguard/internal/application/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/events"`,
		`"github.com/howmuchsec/kiwiguard/internal/policy"`,
		`"github.com/howmuchsec/kiwiguard/internal/verdict"`,
	}

	var offenders []string
	for _, dir := range []string{
		filepath.Join(root, "internal", "domain"),
		filepath.Join(root, "internal", "application"),
	} {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbiddenImport := range forbidden {
				if strings.Contains(string(body), forbiddenImport) {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						rel = path
					}
					offenders = append(offenders, rel)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("domain and application packages must not import infrastructure: %s", strings.Join(offenders, ", "))
	}
}

func TestApplicationLayerDoesNotImportTransportPackages(t *testing.T) {
	root := repositoryRoot(t)
	applicationDirs := []string{
		filepath.Join(root, "internal", "application"),
		filepath.Join(root, "internal", "contexts", "control", "application"),
		filepath.Join(root, "internal", "contexts", "gateway", "application"),
		filepath.Join(root, "internal", "contexts", "runtime", "application"),
	}
	forbidden := []string{
		`"net/http"`,
		`"github.com/go-chi/chi/v5"`,
		`"github.com/go-chi/chi/v5/middleware"`,
		`/adapters/`,
	}

	var offenders []string
	for _, applicationDir := range applicationDirs {
		if _, err := os.Stat(applicationDir); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(applicationDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbiddenImport := range forbidden {
				if strings.Contains(string(body), forbiddenImport) {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						rel = path
					}
					offenders = append(offenders, rel)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("application layer must not import transport packages: %s", strings.Join(offenders, ", "))
	}
}

func TestAppUsesBootstrapInsteadOfRuntimeCompositionRoot(t *testing.T) {
	root := repositoryRoot(t)
	appDir := filepath.Join(root, "internal", "app")
	forbidden := []string{
		`"github.com/howmuchsec/kiwiguard/internal/application/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/runtime"`,
	}

	var offenders []string
	err := filepath.WalkDir(appDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, forbiddenImport := range forbidden {
			if strings.Contains(string(body), forbiddenImport) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("app package must use bootstrap composition root instead of runtime directly: %s", strings.Join(offenders, ", "))
	}
}

func TestRuntimeContextRootContainsOnlyRuntimeStateWatcherWorkerAndPorts(t *testing.T) {
	root := repositoryRoot(t)
	runtimeDir := filepath.Join(root, "internal", "contexts", "runtime")
	allowed := map[string]struct{}{
		"state.go":   {},
		"types.go":   {},
		"watcher.go": {},
		"worker.go":  {},
	}

	var offenders []string
	err := filepath.WalkDir(runtimeDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != runtimeDir {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		name := filepath.Base(path)
		if _, ok := allowed[name]; !ok {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("runtime package must only own state, watcher, worker, and ports: %s", strings.Join(offenders, ", "))
	}
}

func TestRuntimeContextInnerLayersDoNotImportInfrastructureAdapters(t *testing.T) {
	root := repositoryRoot(t)
	runtimeInnerDirs := []string{
		filepath.Join(root, "internal", "contexts", "runtime"),
		filepath.Join(root, "internal", "contexts", "runtime", "application"),
	}
	forbidden := []string{
		`"net/http"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/bootstrap"`,
		`"github.com/howmuchsec/kiwiguard/internal/config"`,
		`"github.com/howmuchsec/kiwiguard/internal/observability"`,
	}

	var offenders []string
	for _, runtimeDir := range runtimeInnerDirs {
		err := filepath.WalkDir(runtimeDir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != runtimeDir {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, token := range forbidden {
				if strings.Contains(string(body), token) {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						rel = path
					}
					offenders = append(offenders, rel)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("runtime context inner layers must not import infrastructure or transport dependencies: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreRecordsLiveInContextPackages(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	retiredFiles := []string{
		"active_revision.go",
		"activation_revision.go",
		"client_records.go",
		"observability_records.go",
		"policy_activation.go",
		"policy_records.go",
		"records.go",
		"revision_records.go",
		"revision_clone.go",
		"revision_draft.go",
		"revision_lookup.go",
		"revision_transactions.go",
		"routing_records.go",
		"runtime_loaders.go",
		"runtime_config.go",
	}
	for _, name := range retiredFiles {
		path := filepath.Join(configstoreDir, name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("configstore root must keep record ownership in context packages, found retired file %s", path)
		}
	}

	if _, err := os.Stat(filepath.Join(configstoreDir, "revision")); !os.IsNotExist(err) {
		t.Fatalf("configstore revision package must move to neutral revisionstore package")
	}
	revisionRecords := filepath.Join(root, "internal", "adapters", "postgres", "revisionstore", "records.go")
	if _, err := os.Stat(revisionRecords); err != nil {
		t.Fatalf("revision records must live in neutral revisionstore package: %v", err)
	}

	policyRecords := filepath.Join(root, "internal", "contexts", "policy", "adapters", "postgres", "records.go")
	if _, err := os.Stat(policyRecords); err != nil {
		t.Fatalf("policy PostgreSQL records must live in the policy context adapter: %v", err)
	}
	routingRecords := filepath.Join(root, "internal", "contexts", "routing", "adapters", "postgres", "records.go")
	if _, err := os.Stat(routingRecords); err != nil {
		t.Fatalf("routing PostgreSQL records must live in the routing context adapter: %v", err)
	}
	clientRecords := filepath.Join(root, "internal", "contexts", "clients", "adapters", "postgres", "records.go")
	if _, err := os.Stat(clientRecords); err != nil {
		t.Fatalf("client PostgreSQL records must live in the clients context adapter: %v", err)
	}
	limitRecords := filepath.Join(root, "internal", "contexts", "clients", "adapters", "postgres", "limit", "records.go")
	if _, err := os.Stat(limitRecords); err != nil {
		t.Fatalf("limit PostgreSQL records must live in the clients context adapter: %v", err)
	}
	observabilityRecords := filepath.Join(root, "internal", "contexts", "traffic", "adapters", "postgres", "observability", "records.go")
	if _, err := os.Stat(observabilityRecords); err != nil {
		t.Fatalf("observability PostgreSQL records must live in the traffic context adapter: %v", err)
	}
}

func TestRetiredConfigStoreContextPersistencePackagesDoNotExist(t *testing.T) {
	root := repositoryRoot(t)
	retiredDirs := []string{
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "client"),
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "limit"),
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "observability"),
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "policy"),
		filepath.Join(root, "internal", "adapters", "postgres", "configstore", "routing"),
	}
	for _, retiredDir := range retiredDirs {
		if _, err := os.Stat(retiredDir); !os.IsNotExist(err) {
			t.Fatalf("context persistence must live under internal/contexts/*/adapters/postgres, found retired directory %s", retiredDir)
		}
	}
}

func TestConfigStoreRoutingSQLLivesInRoutingPackage(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	files := []string{
		filepath.Join(configstoreDir, "routing_methods.go"),
		filepath.Join(configstoreDir, "routing_clone.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_loaders.go"),
	}
	forbiddenInRoot := []string{
		"from routes",
		"from providers",
		"from model_mappings",
		"from verdict_providers",
		"from route_verdict_provider_bindings",
		"insert into routes",
		"insert into providers",
		"insert into model_mappings",
		"insert into verdict_providers",
		"insert into route_verdict_provider_bindings",
	}

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenInRoot {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("routing SQL text must live in the routing context PostgreSQL adapter: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreLimitSQLLivesInLimitPackage(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	files := []string{
		filepath.Join(configstoreDir, "limit_methods.go"),
		filepath.Join(configstoreDir, "routing_clone.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_loaders.go"),
	}
	forbiddenInRoot := []string{
		"from route_limit_policies",
		"from client_route_limit_overrides",
		"insert into route_limit_policies",
		"insert into client_route_limit_overrides",
		"delete from client_route_limit_overrides",
	}

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenInRoot {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("limit SQL text must live in the clients context PostgreSQL adapter: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStorePolicySQLLivesInPolicyPackage(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	files := []string{
		filepath.Join(configstoreDir, "activation_policy.go"),
		filepath.Join(configstoreDir, "policy_bundle_loaders.go"),
		filepath.Join(configstoreDir, "policy_clone.go"),
		filepath.Join(configstoreDir, "policy_mutations.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_repository.go"),
		filepath.Join(configstoreDir, "revision_lookup.go"),
		filepath.Join(configstoreDir, "revision_draft.go"),
		filepath.Join(configstoreDir, "revision_clone_orchestration.go"),
	}
	forbiddenInRoot := []string{
		"from policy_bundles",
		"from policy_detectors",
		"from policy_rules",
		"from policy_rule_detectors",
		"from policy_rule_scopes",
		"from route_policy_bindings",
		"insert into policy_bundles",
		"insert into policy_detectors",
		"insert into policy_rules",
		"insert into policy_rule_detectors",
		"insert into policy_rule_scopes",
		"insert into route_policy_bindings",
		"delete from policy_detectors",
		"delete from policy_rules",
	}

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenInRoot {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("policy SQL text must live in the policy context PostgreSQL adapter: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreObservabilitySQLLivesInObservabilityPackage(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	files := []string{
		filepath.Join(configstoreDir, "observability_clone.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_loaders.go"),
		filepath.Join(configstoreDir, "revision_lookup.go"),
		filepath.Join(configstoreDir, "revision_draft.go"),
		filepath.Join(configstoreDir, "revision_clone_orchestration.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_repository.go"),
	}
	forbiddenInRoot := []string{
		"from sinks",
		"from retention_policies",
		"from raw_capture_policies",
		"insert into sinks",
		"insert into retention_policies",
		"insert into raw_capture_policies",
	}

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenInRoot {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("observability SQL text must live in the traffic context PostgreSQL adapter: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreClientSQLLivesInClientPackage(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	files := []string{
		filepath.Join(configstoreDir, "client_methods.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_loaders.go"),
		filepath.Join(configstoreDir, "runtime_aggregate_repository.go"),
	}
	forbiddenInRoot := []string{
		"from gateway_clients",
		"insert into gateway_clients",
		"update gateway_clients",
	}

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbiddenInRoot {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("gateway client SQL text must live in the clients context PostgreSQL adapter: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreProductionCodeDoesNotExposeBusinessFacadeMethods(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	forbidden := []string{
		"func (r *ConfigRepository) ListPolicyBundles(",
		"func (r *ConfigRepository) UpsertPolicyBundle(",
		"func (r *ConfigRepository) ActivatePolicyBundles(",
		"func (r *ConfigRepository) LoadActiveRuntimeConfig(",
		"func (r *ConfigRepository) ListRoutes(",
		"func (r *ConfigRepository) ListProviders(",
		"func (r *ConfigRepository) ListModelMappings(",
		"func (r *ConfigRepository) UpsertModelMapping(",
		"func (r *ConfigRepository) ListVerdictProviders(",
		"func (r *ConfigRepository) UpsertVerdictProvider(",
		"func (r *ConfigRepository) ListGatewayClients(",
		"func (r *ConfigRepository) CreateGatewayClient(",
		"func (r *ConfigRepository) UpsertGatewayClient(",
		"func (r *ConfigRepository) RevokeGatewayClient(",
		"func (r *ConfigRepository) ListRouteLimitPolicies(",
		"func (r *ConfigRepository) UpsertRouteLimitPolicy(",
		"func (r *ConfigRepository) ListClientRouteLimitOverrides(",
		"func (r *ConfigRepository) UpsertClientRouteLimitOverride(",
		"func (r *ConfigRepository) DeleteClientRouteLimitOverride(",
	}

	var offenders []string
	err := filepath.WalkDir(configstoreDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, token := range forbidden {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("configstore production code must expose shared revision primitives, not business facades: %s", strings.Join(offenders, ", "))
	}
}

func TestGatewayContextDomainAndApplicationDoNotImportInfrastructureOrTransport(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		`"net/http"`,
		`"github.com/go-chi/chi/v5"`,
		`"github.com/go-chi/chi/v5/middleware"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/bootstrap"`,
		`"github.com/howmuchsec/kiwiguard/internal/config"`,
		`"github.com/howmuchsec/kiwiguard/internal/infrastructure/`,
		`"github.com/howmuchsec/kiwiguard/internal/observability"`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/runtime"`,
		`"github.com/howmuchsec/kiwiguard/internal/transport/`,
	}
	dirs := gatewayContextInnerLayerDirs(root)

	offenders := filesContainingAnyImport(t, root, dirs, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("gateway context domain and application packages must not import infrastructure or transport: %s", strings.Join(offenders, ", "))
	}
}

func TestTrafficContextDomainDoesNotImportInfrastructureOrTransport(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		`"net/http"`,
		`"github.com/go-chi/chi/v5"`,
		`"github.com/go-chi/chi/v5/middleware"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/bootstrap"`,
		`"github.com/howmuchsec/kiwiguard/internal/config"`,
		`"github.com/howmuchsec/kiwiguard/internal/infrastructure/`,
		`"github.com/howmuchsec/kiwiguard/internal/observability"`,
		`"github.com/howmuchsec/kiwiguard/internal/transport/`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/traffic/adapters`,
	}
	dirs := []string{
		filepath.Join(root, "internal", "contexts", "traffic", "domain"),
	}

	offenders := filesContainingAnyImport(t, root, dirs, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("traffic context domain must not import infrastructure or transport: %s", strings.Join(offenders, ", "))
	}
}

func TestVerdictContextDomainDoesNotImportInfrastructureOrTransport(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		`"net/http"`,
		`"github.com/go-chi/chi/v5"`,
		`"github.com/go-chi/chi/v5/middleware"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/bootstrap"`,
		`"github.com/howmuchsec/kiwiguard/internal/config"`,
		`"github.com/howmuchsec/kiwiguard/internal/infrastructure/`,
		`"github.com/howmuchsec/kiwiguard/internal/observability"`,
		`"github.com/howmuchsec/kiwiguard/internal/transport/`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/verdict/adapters`,
	}
	dirs := []string{
		filepath.Join(root, "internal", "contexts", "verdict", "domain"),
	}

	offenders := filesContainingAnyImport(t, root, dirs, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("verdict context domain must not import infrastructure or transport: %s", strings.Join(offenders, ", "))
	}
}

func TestCoreContextDomainsDoNotImportInfrastructureOrTransport(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		`"net/http"`,
		`"github.com/go-chi/chi/v5"`,
		`"github.com/go-chi/chi/v5/middleware"`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/`,
		`"github.com/howmuchsec/kiwiguard/internal/bootstrap"`,
		`"github.com/howmuchsec/kiwiguard/internal/config"`,
		`"github.com/howmuchsec/kiwiguard/internal/infrastructure/`,
		`"github.com/howmuchsec/kiwiguard/internal/observability"`,
		`"github.com/howmuchsec/kiwiguard/internal/transport/`,
	}
	dirs := []string{
		filepath.Join(root, "internal", "contexts", "clients", "domain"),
		filepath.Join(root, "internal", "contexts", "detection", "domain"),
		filepath.Join(root, "internal", "contexts", "policy", "domain"),
		filepath.Join(root, "internal", "contexts", "routing", "domain"),
	}

	offenders := filesContainingAnyImport(t, root, dirs, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("core context domains must not import infrastructure or transport: %s", strings.Join(offenders, ", "))
	}
}

func TestGatewayAdapterIsNotImportedByDomainOrApplication(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		`"github.com/howmuchsec/kiwiguard/internal/adapters/gatewayruntime`,
		`"github.com/howmuchsec/kiwiguard/internal/adapters/http/gateway`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapter`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/gateway/adapters`,
	}
	dirs := append([]string{
		filepath.Join(root, "internal", "domain"),
		filepath.Join(root, "internal", "application"),
	}, gatewayContextInnerLayerDirs(root)...)

	offenders := filesContainingAnyImport(t, root, dirs, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("domain and application packages must not import gateway adapters: %s", strings.Join(offenders, ", "))
	}
}

func TestGatewayOpenAIHandlerDoesNotOwnGatewayBusinessDecisions(t *testing.T) {
	root := repositoryRoot(t)
	handler := filepath.Join(root, "internal", "contexts", "gateway", "adapters", "http", "openai", "openai_handler.go")
	forbidden := []string{
		`"github.com/howmuchsec/kiwiguard/internal/contexts/detection/domain"`,
		`"github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"`,
		"policy.Decision",
		"detection.Direction",
		"ResolveModelMapping(",
		"text/event-stream",
		"handleOpenAIStream",
		"writeOpenAIError",
		"prepareOpenAIExchange",
		"applyOpenAIInputPolicy",
		"projectOpenAIUpstreamRequest",
		"forwardOpenAIExchange",
	}

	offenders := filesContainingAnyToken(t, root, []string{handler}, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("OpenAI handler must stay a protocol coordinator and delegate gateway decisions: %s", strings.Join(offenders, ", "))
	}
}

func TestGatewayOpenAIAdapterDelegatesLifecycleDecisionsToApplication(t *testing.T) {
	root := repositoryRoot(t)
	openAIDir := filepath.Join(root, "internal", "contexts", "gateway", "adapters", "http", "openai")
	files := []string{
		filepath.Join(openAIDir, "request_admission.go"),
		filepath.Join(openAIDir, "openai_exchange.go"),
		filepath.Join(openAIDir, "openai_policy.go"),
		filepath.Join(openAIDir, "openai_upstream_request.go"),
		filepath.Join(openAIDir, "streaming_handler.go"),
	}
	forbidden := []string{
		"RouteAvailable(",
		"ClassifyExchangeInfrastructure(",
		"ClassifyDecodedRequest(",
		"ClassifyInputRequest(",
		"ClassifyOutputResponse(",
		"ClassifyStreamingDelta(",
		"ExchangeInfrastructureInput{",
		"DecodedRequestInput{",
		"InputRequestLifecycleInput{",
		"OutputResponseLifecycleInput{",
		"StreamingDeltaInput{",
	}

	offenders := filesContainingAnyToken(t, root, files, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("OpenAI adapter lifecycle files must use application planners instead of owning business decisions: %s", strings.Join(offenders, ", "))
	}
}

func TestGatewayOpenAIAdapterDoesNotOwnRuntimeCompilerOrStreamState(t *testing.T) {
	root := repositoryRoot(t)
	openAIDir := filepath.Join(root, "internal", "contexts", "gateway", "adapters", "http", "openai")
	forbidden := []string{
		"func CompileGatewayRuntime",
		"type GatewayRuntimeCompiler",
		"type Compiler struct",
		"type StreamWindow",
		"type SlidingWindow",
		"type StreamTextCollector",
		"type boundedTextCollector",
	}

	var files []string
	err := filepath.WalkDir(openAIDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	offenders := filesContainingAnyToken(t, root, files, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("OpenAI adapter must not own runtime compiler or streaming lifecycle state: %s", strings.Join(offenders, ", "))
	}
}

func TestRetiredGatewayMigrationPackagesDoNotExist(t *testing.T) {
	root := repositoryRoot(t)
	retiredDirs := []string{
		filepath.Join(root, "internal", "application", "gateway"),
		filepath.Join(root, "internal", "adapters", "http", "gateway"),
		filepath.Join(root, "internal", "adapters", "gatewayruntime"),
	}

	for _, retiredDir := range retiredDirs {
		if _, err := os.Stat(retiredDir); !os.IsNotExist(err) {
			t.Fatalf("retired gateway migration package directory must not exist: %s", retiredDir)
		}
	}
}

func TestBootstrapDoesNotUseCentralConfigRepositoryBusinessFacade(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		"NewConfigRepository(",
		".Control()",
		".Runtime()",
		".Policy()",
		".Routing()",
		".Clients()",
		".Limits()",
	}
	offenders := filesContainingAnyTokenInDir(t, root, filepath.Join(root, "internal", "bootstrap"), forbidden)
	if len(offenders) > 0 {
		t.Fatalf("bootstrap must use context-owned persistence adapters instead of configstore business facade: %s", strings.Join(offenders, ", "))
	}
}

func TestContextPostgresAdaptersDoNotUseConfigstoreBusinessFacade(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []string{
		".Control()",
		".Runtime()",
		".Policy()",
		".Routing()",
		".Clients()",
		".Limits()",
	}
	dirs := []string{
		filepath.Join(root, "internal", "contexts", "control", "adapters", "postgres"),
		filepath.Join(root, "internal", "contexts", "runtime", "adapters", "postgres"),
	}

	var offenders []string
	for _, dir := range dirs {
		offenders = append(offenders, filesContainingAnyTokenInDir(t, root, dir, forbidden)...)
	}
	if len(offenders) > 0 {
		t.Fatalf("context PostgreSQL adapters must bind directly to narrow ports instead of configstore business facade methods: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreRootDoesNotExposeBusinessFacade(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	forbidden := []string{
		"type RuntimeConfig =",
		"type PolicyBundle =",
		"type Detector =",
		"type Rule =",
		"type RuleScope =",
		"type Route =",
		"type Provider =",
		"type ModelMapping =",
		"type VerdictProvider =",
		"type RouteVerdictProviderBinding =",
		"type GatewayClient =",
		"type RouteLimitPolicy =",
		"type ClientRouteLimitOverride =",
		"type Sink =",
		"type RetentionPolicy =",
		"type RawCapturePolicy =",
		"type Queryer =",
		"type ConfigRevision =",
		"type RevisionActivationRequest =",
		"type RevisionActivationHooks =",
		"type RevisionActivationRecord =",
		"type RevisionActivationResult =",
		"type ControlStore",
		"type RuntimeStore",
		"type PolicyStore",
		"type RoutingStore",
		"type ClientStore",
		"type LimitStore",
		"type ActivatePolicyBundlesRequest =",
		"type ActivationResult =",
		"func (r *ConfigRepository) Control()",
		"func (r *ConfigRepository) Runtime()",
		"func (r *ConfigRepository) Policy()",
		"func (r *ConfigRepository) Routing()",
		"func (r *ConfigRepository) Clients()",
		"func (r *ConfigRepository) Limits()",
	}
	offenders := filesContainingAnyTokenInDir(t, root, configstoreDir, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("configstore root must not expose cross-context business facade APIs: %s", strings.Join(offenders, ", "))
	}
}

func TestRevisionStoreDoesNotOwnPolicyActivationDTOs(t *testing.T) {
	root := repositoryRoot(t)
	revisionstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "revisionstore")
	forbidden := []string{
		"type ActivatePolicyBundlesRequest",
		"type ActivationResult struct",
		"ActiveKeys",
	}
	offenders := filesContainingAnyTokenInDir(t, root, revisionstoreDir, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("revisionstore must own revision unit-of-work primitives, not policy activation DTOs: %s", strings.Join(offenders, ", "))
	}
}

func TestConfigStoreRootDoesNotRegrowBusinessRepositoryMethods(t *testing.T) {
	root := repositoryRoot(t)
	configstoreDir := filepath.Join(root, "internal", "adapters", "postgres", "configstore")
	forbidden := []string{
		"*ConfigRepository) LoadActiveRuntimeConfig(",
		"*ConfigRepository) ActivatePolicyBundles(",
		"*ConfigRepository) List",
		"*ConfigRepository) Upsert",
		"*ConfigRepository) Create",
		"*ConfigRepository) Revoke",
		"*ConfigRepository) Delete",
	}
	offenders := filesContainingAnyTokenInDir(t, root, configstoreDir, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("configstore root must expose shared revision primitives, not context business repository methods: %s", strings.Join(offenders, ", "))
	}
}

func TestProductionConfigstoreImportsStayAtCompositionOrContextAdapterBoundary(t *testing.T) {
	root := repositoryRoot(t)
	internalDir := filepath.Join(root, "internal")
	configstoreImport := `"github.com/howmuchsec/kiwiguard/internal/adapters/postgres/configstore`

	var offenders []string
	err := filepath.WalkDir(internalDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), configstoreImport) || allowedProductionConfigstoreImporter(root, path) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		offenders = append(offenders, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("production code must reach configstore only from bootstrap, CLI migrations, or context-owned PostgreSQL adapters: %s", strings.Join(offenders, ", "))
	}
}

func TestContextProductionCodeDoesNotUseConfigstoreFacadeRecords(t *testing.T) {
	root := repositoryRoot(t)
	contextsDir := filepath.Join(root, "internal", "contexts")
	forbidden := []string{
		"configstore.RuntimeConfig",
		"configstore.PolicyBundle",
		"configstore.Detector",
		"configstore.Rule",
		"configstore.RuleScope",
		"configstore.Route",
		"configstore.Provider",
		"configstore.ModelMapping",
		"configstore.VerdictProvider",
		"configstore.RouteLimitPolicy",
		"configstore.ClientRouteLimitOverride",
		"configstore.Sink",
		"configstore.RetentionPolicy",
		"configstore.RawCapturePolicy",
		"configstore.ActivatePolicyBundlesRequest",
		"configstore.ActivationResult",
	}
	offenders := filesContainingAnyTokenInDir(t, root, contextsDir, forbidden)
	if len(offenders) > 0 {
		t.Fatalf("context production code must depend on context-owned PostgreSQL records, not configstore facade records: %s", strings.Join(offenders, ", "))
	}
}

func TestBootstrapRootDoesNotRegrowFactoryOrProductionAssemblyFiles(t *testing.T) {
	root := repositoryRoot(t)
	bootstrapDir := filepath.Join(root, "internal", "bootstrap")
	forbiddenPrefixes := []string{
		"factory_",
		"production_",
	}
	forbiddenNames := map[string]bool{
		"factory.go":                true,
		"resources_dependencies.go": true,
	}

	var offenders []string
	entries, err := os.ReadDir(bootstrapDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		forbidden := forbiddenNames[entry.Name()]
		for _, prefix := range forbiddenPrefixes {
			forbidden = forbidden || strings.HasPrefix(entry.Name(), prefix)
		}
		if forbidden {
			offenders = append(offenders, filepath.Join("internal", "bootstrap", entry.Name()))
		}
	}
	if len(offenders) > 0 {
		t.Fatalf("bootstrap root must use explicit assembly names instead of factory_ or production_ catch-all files: %s", strings.Join(offenders, ", "))
	}
}

func gatewayContextInnerLayerDirs(root string) []string {
	return []string{
		filepath.Join(root, "internal", "contexts", "gateway", "domain"),
		filepath.Join(root, "internal", "contexts", "gateway", "application"),
	}
}

func allowedProductionConfigstoreImporter(root string, path string) bool {
	allowedDirs := []string{
		filepath.Join(root, "internal", "adapters", "postgres", "configstore"),
		filepath.Join(root, "internal", "bootstrap", "resources"),
		filepath.Join(root, "internal", "contexts", "control", "adapters", "postgres"),
		filepath.Join(root, "internal", "contexts", "runtime", "adapters", "postgres"),
	}
	for _, dir := range allowedDirs {
		if strings.HasPrefix(path, dir+string(os.PathSeparator)) {
			return true
		}
	}
	return path == filepath.Join(root, "internal", "cli", "root.go")
}

func filesContainingAnyImport(t *testing.T, root string, dirs []string, forbidden []string) []string {
	t.Helper()

	var offenders []string
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, forbiddenImport := range forbidden {
				if strings.Contains(string(body), forbiddenImport) {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						rel = path
					}
					offenders = append(offenders, rel)
					break
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return offenders
}

func filesContainingAnyTokenInDir(t *testing.T, root string, dir string, forbidden []string) []string {
	t.Helper()

	var files []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return filesContainingAnyToken(t, root, files, forbidden)
}

func filesContainingAnyToken(t *testing.T, root string, files []string, forbidden []string) []string {
	t.Helper()

	var offenders []string
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range forbidden {
			if strings.Contains(string(body), token) {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					rel = path
				}
				offenders = append(offenders, rel)
				break
			}
		}
	}
	return offenders
}

func retiredGatewayImport() string {
	return `"github.com/howmuchsec/kiwiguard/internal/` + `gateway"`
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
