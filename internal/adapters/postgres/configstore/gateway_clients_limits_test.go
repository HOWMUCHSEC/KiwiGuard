package configstore

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	clientstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres"
	limitstore "github.com/howmuchsec/kiwiguard/internal/contexts/clients/adapters/postgres/limit"
	policystore "github.com/howmuchsec/kiwiguard/internal/contexts/policy/adapters/postgres"
	policy "github.com/howmuchsec/kiwiguard/internal/contexts/policy/domain"
)

func TestConfigRepositoryGatewayClientsAndLimitsLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)
	seedOpenAIRoute(t, ctx, pool)

	client := clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "enabled",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
		Notes:      "primary test client",
	}
	if err := testUpsertGatewayClient(ctx, repo, client); err != nil {
		t.Fatalf("UpsertGatewayClient() error = %v", err)
	}

	routeID := routeIDByName(t, ctx, repo, "openai")
	policy := limitstore.RoutePolicy{
		RouteID:               routeID,
		RequestsPerWindow:     120,
		WindowSeconds:         60,
		MaxConcurrentRequests: 8,
		MaxBodyBytes:          1_048_576,
		Enabled:               true,
	}
	if err := testUpsertRouteLimitPolicy(ctx, repo, policy); err != nil {
		t.Fatalf("UpsertRouteLimitPolicy() error = %v", err)
	}

	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(clients))
	}
	client = clients[0]
	if client.ID == "" || client.ExternalID != "client-acme" || client.Name != "Acme Console" || client.Status != "enabled" || client.KeyPrefix != "kg_acme" || client.KeyHash != "sha256:acme" || client.Notes != "primary test client" || client.CreatedAt.IsZero() || client.UpdatedAt.IsZero() || client.RevokedAt != nil {
		t.Fatalf("GatewayClients[0] = %+v, want enabled Acme client with timestamps", client)
	}

	override := limitstore.ClientRouteOverride{
		ClientID:              client.ID,
		RouteID:               routeIDByName(t, ctx, repo, "openai"),
		RequestsPerWindow:     40,
		WindowSeconds:         60,
		MaxConcurrentRequests: 3,
		MaxBodyBytes:          262_144,
		Enabled:               true,
	}
	if err := testUpsertClientRouteLimitOverride(ctx, repo, override); err != nil {
		t.Fatalf("UpsertClientRouteLimitOverride() error = %v", err)
	}

	policies, err := testListRouteLimitPolicies(ctx, repo)
	if err != nil {
		t.Fatalf("ListRouteLimitPolicies() error = %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("RouteLimitPolicies len = %d, want 1", len(policies))
	}
	gotPolicy := policies[0]
	if gotPolicy.ID == "" || gotPolicy.RouteID == "" || gotPolicy.RequestsPerWindow != 120 || gotPolicy.WindowSeconds != 60 || gotPolicy.MaxConcurrentRequests != 8 || gotPolicy.MaxBodyBytes != 1_048_576 || !gotPolicy.Enabled {
		t.Fatalf("RouteLimitPolicies[0] = %+v, want openai default limit", gotPolicy)
	}

	overrides, err := testListClientRouteLimitOverrides(ctx, repo, client.ID)
	if err != nil {
		t.Fatalf("ListClientRouteLimitOverrides() error = %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("ClientRouteLimitOverrides len = %d, want 1", len(overrides))
	}
	gotOverride := overrides[0]
	if gotOverride.ID == "" || gotOverride.ClientID != client.ID || gotOverride.RouteID == "" || gotOverride.RequestsPerWindow != 40 || gotOverride.WindowSeconds != 60 || gotOverride.MaxConcurrentRequests != 3 || gotOverride.MaxBodyBytes != 262_144 || !gotOverride.Enabled {
		t.Fatalf("ClientRouteLimitOverrides[0] = %+v, want Acme openai override", gotOverride)
	}
}

func TestConfigRepositoryCreateGatewayClientDoesNotUpdateDuplicate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	if err := testCreateGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "enabled",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
	}); err != nil {
		t.Fatalf("CreateGatewayClient() error = %v", err)
	}

	err := testCreateGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Rotated",
		Status:     "enabled",
		KeyPrefix:  "kg_acme_rotated",
		KeyHash:    "sha256:rotated",
	})
	if !errors.Is(err, ErrGatewayClientAlreadyExists) {
		t.Fatalf("CreateGatewayClient() duplicate error = %v, want ErrGatewayClientAlreadyExists", err)
	}

	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(clients))
	}
	if clients[0].Name != "Acme Console" || clients[0].KeyPrefix != "kg_acme" || clients[0].KeyHash != "sha256:acme" {
		t.Fatalf("GatewayClients[0] = %+v, want original credentials preserved", clients[0])
	}
}

func TestConfigRepositoryGatewayClientMutationsAdvanceRuntimeRevision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	listenerPool := newMigratedPostgresPoolFromDSN(t, ctx, pool.Config().ConnString())
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	conn, err := listenerPool.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "listen "+ConfigActivatedChannel); err != nil {
		t.Fatalf("listen %s: %v", ConfigActivatedChannel, err)
	}

	before, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() before client upsert error = %v", err)
	}
	if err := testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "enabled",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
	}); err != nil {
		t.Fatalf("UpsertGatewayClient() error = %v", err)
	}
	afterUpsert, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() after client upsert error = %v", err)
	}
	if afterUpsert <= before {
		t.Fatalf("ActiveRevisionNumber after upsert = %d, want greater than %d", afterUpsert, before)
	}
	notification, err := conn.Conn().WaitForNotification(ctx)
	if err != nil {
		t.Fatalf("WaitForNotification() after upsert error = %v", err)
	}
	if notification.Channel != ConfigActivatedChannel || notification.Payload != strconv.FormatInt(afterUpsert, 10) {
		t.Fatalf("upsert notification = %s/%s, want %s/%d", notification.Channel, notification.Payload, ConfigActivatedChannel, afterUpsert)
	}
	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if cfg.Revision.Number != afterUpsert {
		t.Fatalf("RuntimeConfig revision = %d, want active revision token %d", cfg.Revision.Number, afterUpsert)
	}

	if err := testRevokeGatewayClient(ctx, repo, "client-acme"); err != nil {
		t.Fatalf("RevokeGatewayClient() error = %v", err)
	}
	afterRevoke, err := repo.ActiveRevisionNumber(ctx)
	if err != nil {
		t.Fatalf("ActiveRevisionNumber() after revoke error = %v", err)
	}
	if afterRevoke <= afterUpsert {
		t.Fatalf("ActiveRevisionNumber after revoke = %d, want greater than %d", afterRevoke, afterUpsert)
	}
	notification, err = conn.Conn().WaitForNotification(ctx)
	if err != nil {
		t.Fatalf("WaitForNotification() after revoke error = %v", err)
	}
	if notification.Channel != ConfigActivatedChannel || notification.Payload != strconv.FormatInt(afterRevoke, 10) {
		t.Fatalf("revoke notification = %s/%s, want %s/%d", notification.Channel, notification.Payload, ConfigActivatedChannel, afterRevoke)
	}
}

func TestConfigRepositoryGatewayClientUpsertDoesNotReviveRevokedClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)

	if err := testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Console",
		Status:     "enabled",
		KeyPrefix:  "kg_acme",
		KeyHash:    "sha256:acme",
	}); err != nil {
		t.Fatalf("UpsertGatewayClient() error = %v", err)
	}
	if err := testRevokeGatewayClient(ctx, repo, "client-acme"); err != nil {
		t.Fatalf("RevokeGatewayClient() error = %v", err)
	}
	revoked, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() after revoke error = %v", err)
	}
	if len(revoked) != 1 || revoked[0].Status != "revoked" || revoked[0].RevokedAt == nil {
		t.Fatalf("GatewayClients after revoke = %+v, want revoked client with timestamp", revoked)
	}

	if err := testUpsertGatewayClient(ctx, repo, clientstore.GatewayClient{
		ExternalID: "client-acme",
		Name:       "Acme Renamed",
		Status:     "enabled",
		KeyPrefix:  "kg_acme_rotated",
		KeyHash:    "sha256:rotated",
	}); err != nil {
		t.Fatalf("UpsertGatewayClient() after revoke error = %v", err)
	}
	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() after revive attempt error = %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(clients))
	}
	if clients[0].Status != "revoked" || clients[0].RevokedAt == nil || !clients[0].RevokedAt.Equal(*revoked[0].RevokedAt) {
		t.Fatalf("GatewayClients[0] = %+v, want revoked status and original revoke timestamp", clients[0])
	}
}

func TestConfigRepositoryLoadsGatewayClientsAndLimitsInActiveRuntimeConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)
	seedOpenAIRoute(t, ctx, pool)
	seedGatewayClientAndLimits(t, ctx, pool)

	cfg, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() error = %v", err)
	}
	if len(cfg.GatewayClients) != 1 || cfg.GatewayClients[0].ExternalID != "client-acme" || cfg.GatewayClients[0].Status != "enabled" {
		t.Fatalf("GatewayClients = %+v, want active Acme client", cfg.GatewayClients)
	}
	if len(cfg.RouteLimitPolicies) != 1 || cfg.RouteLimitPolicies[0].RequestsPerWindow != 120 || cfg.RouteLimitPolicies[0].RouteID == "" {
		t.Fatalf("RouteLimitPolicies = %+v, want one openai route policy", cfg.RouteLimitPolicies)
	}
	if len(cfg.ClientRouteLimitOverrides) != 1 || cfg.ClientRouteLimitOverrides[0].ClientID != cfg.GatewayClients[0].ID || cfg.ClientRouteLimitOverrides[0].RequestsPerWindow != 40 {
		t.Fatalf("ClientRouteLimitOverrides = %+v, want one Acme override", cfg.ClientRouteLimitOverrides)
	}
}

func TestConfigRepositoryClonesGatewayLimitRecordsIntoDraft(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)
	seedOpenAIRoute(t, ctx, pool)
	seedGatewayClientAndLimits(t, ctx, pool)

	if err := testUpsertPolicyBundle(ctx, repo, policystore.Bundle{
		Key:           "limits-clone-trigger",
		Version:       "2026.06",
		Source:        string(policy.SourceUser),
		DefaultAction: string(policy.ActionAllow),
		Enabled:       true,
	}); err != nil {
		t.Fatalf("UpsertPolicyBundle() error = %v", err)
	}

	policies, err := testListRouteLimitPolicies(ctx, repo)
	if err != nil {
		t.Fatalf("ListRouteLimitPolicies() error = %v", err)
	}
	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(clients))
	}
	overrides, err := testListClientRouteLimitOverrides(ctx, repo, clients[0].ID)
	if err != nil {
		t.Fatalf("ListClientRouteLimitOverrides() error = %v", err)
	}

	if len(policies) != 1 || policies[0].RequestsPerWindow != 120 {
		t.Fatalf("RouteLimitPolicies = %+v, want cloned policy", policies)
	}
	if len(overrides) != 1 || overrides[0].RequestsPerWindow != 40 {
		t.Fatalf("ClientRouteLimitOverrides = %+v, want cloned override", overrides)
	}
	routeID := routeIDByName(t, ctx, repo, "openai")
	if policies[0].RouteID != routeID || overrides[0].RouteID != routeID {
		t.Fatalf("cloned route IDs policy=%q override=%q current=%q", policies[0].RouteID, overrides[0].RouteID, routeID)
	}
}

func TestConfigRepositoryDeletesClientRouteLimitOverrideInDraft(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool := newMigratedPostgresPool(t, ctx)
	repo := NewConfigRepository(pool)
	seedRuntimeConfig(t, ctx, pool)
	seedOpenAIRoute(t, ctx, pool)
	seedGatewayClientAndLimits(t, ctx, pool)

	active, err := testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() before delete error = %v", err)
	}
	if len(active.ClientRouteLimitOverrides) != 1 {
		t.Fatalf("active ClientRouteLimitOverrides before delete = %+v, want one override", active.ClientRouteLimitOverrides)
	}

	clients, err := testListGatewayClients(ctx, repo)
	if err != nil {
		t.Fatalf("ListGatewayClients() error = %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("GatewayClients len = %d, want 1", len(clients))
	}
	activeRouteID := routeIDByName(t, ctx, repo, "openai")
	if err := testDeleteClientRouteLimitOverride(ctx, repo, clients[0].ID, activeRouteID); err != nil {
		t.Fatalf("DeleteClientRouteLimitOverride() error = %v", err)
	}

	overrides, err := testListClientRouteLimitOverrides(ctx, repo, clients[0].ID)
	if err != nil {
		t.Fatalf("ListClientRouteLimitOverrides() error = %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("ClientRouteLimitOverrides after delete = %+v, want none in draft", overrides)
	}

	active, err = testLoadActiveRuntimeConfig(ctx, repo)
	if err != nil {
		t.Fatalf("LoadActiveRuntimeConfig() after delete error = %v", err)
	}
	if len(active.ClientRouteLimitOverrides) != 1 {
		t.Fatalf("active ClientRouteLimitOverrides after delete = %+v, want active override preserved", active.ClientRouteLimitOverrides)
	}
}
