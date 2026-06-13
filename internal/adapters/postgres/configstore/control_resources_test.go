package configstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	routingstore "github.com/howmuchsec/kiwiguard/internal/contexts/routing/adapters/postgres"
)

func TestConfigRepositoryListsAndUpsertsControlResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{
		Name:        "default",
		SourceModel: "gpt-test",
		TargetModel: "gpt-4o-mini",
		Parameters:  json.RawMessage(`{"temperature":0}`),
	}); err != nil {
		t.Fatalf("UpsertModelMapping() error = %v", err)
	}
	if err := testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
		Name:           "sec-model",
		Adapter:        "http",
		Endpoint:       "http://verdict.test/evaluate",
		Timeout:        time.Second,
		MaxConcurrency: 8,
		Enabled:        true,
	}); err != nil {
		t.Fatalf("UpsertVerdictProvider() error = %v", err)
	}

	mappings, err := testListModelMappings(ctx, repo)
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if len(mappings) != 1 || mappings[0].Name != "default" || mappings[0].SourceModel != "gpt-test" {
		t.Fatalf("ModelMappings = %+v, want default gpt-test mapping", mappings)
	}

	providers, err := testListVerdictProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 1 || providers[0].Name != "sec-model" || providers[0].MaxConcurrency != 8 {
		t.Fatalf("VerdictProviders = %+v, want sec-model provider", providers)
	}
}

func TestConfigRepositoryPersistsDisabledVerdictProvider(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
		Name:     "disabled-sec-model",
		Adapter:  "http",
		Endpoint: "http://verdict.test/evaluate",
		Enabled:  false,
	}); err != nil {
		t.Fatalf("UpsertVerdictProvider() error = %v", err)
	}

	providers, err := testListVerdictProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("VerdictProviders len = %d, want 1", len(providers))
	}
	if providers[0].Enabled {
		t.Fatalf("VerdictProviders[0].Enabled = true, want false")
	}
}

func TestConfigRepositoryRollsBackInvalidControlResourceUpserts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	err := testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{
		Name:             "bad-mapping",
		SourceModel:      "gpt-test",
		TargetProviderID: "not-a-uuid",
		TargetModel:      "gpt-4o-mini",
	})
	if err == nil {
		t.Fatal("UpsertModelMapping() error = nil, want invalid UUID error")
	}
	if !strings.Contains(err.Error(), "upsert model mapping") {
		t.Fatalf("UpsertModelMapping() error = %v, want upsert context", err)
	}
	if _, err := testListModelMappings(ctx, repo); !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("ListModelMappings() error = %v, want ErrActiveConfigNotFound after rollback", err)
	}

	err = testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
		Name:          "bad-provider",
		Endpoint:      "http://verdict.test/evaluate",
		AdapterConfig: json.RawMessage(`{bad-json}`),
	})
	if err == nil {
		t.Fatal("UpsertVerdictProvider() error = nil, want invalid JSON error")
	}
	if !strings.Contains(err.Error(), "upsert verdict provider") {
		t.Fatalf("UpsertVerdictProvider() error = %v, want upsert context", err)
	}
	if _, err := testListVerdictProviders(ctx, repo); !errors.Is(err, ErrActiveConfigNotFound) {
		t.Fatalf("ListVerdictProviders() error = %v, want ErrActiveConfigNotFound after rollback", err)
	}
}

func TestConfigRepositoryUpsertsControlResourcesWithDefaults(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)

	if err := testUpsertModelMapping(ctx, repo, routingstore.ModelMapping{
		Name:        "default",
		SourceModel: "gpt-requested",
		TargetModel: "gpt-upstream",
	}); err != nil {
		t.Fatalf("UpsertModelMapping() error = %v", err)
	}
	if err := testUpsertVerdictProvider(ctx, repo, routingstore.VerdictProvider{
		Name:     "sec-model",
		Endpoint: "http://verdict.test/evaluate",
		Enabled:  false,
	}); err != nil {
		t.Fatalf("UpsertVerdictProvider() error = %v", err)
	}

	mappings, err := testListModelMappings(ctx, repo)
	if err != nil {
		t.Fatalf("ListModelMappings() error = %v", err)
	}
	if len(mappings) != 1 || string(mappings[0].Parameters) != "{}" {
		t.Fatalf("ModelMappings = %+v, want default empty parameters", mappings)
	}
	providers, err := testListVerdictProviders(ctx, repo)
	if err != nil {
		t.Fatalf("ListVerdictProviders() error = %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("VerdictProviders len = %d, want 1", len(providers))
	}
	provider := providers[0]
	if provider.Adapter != "http" || provider.Timeout != 5*time.Second || provider.MaxConcurrency != 16 || provider.Enabled {
		t.Fatalf("VerdictProvider = %+v, want default adapter/timeout/concurrency and disabled state", provider)
	}
}
