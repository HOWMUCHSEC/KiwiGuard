package openai

import "testing"

func TestGenerateClientKeyReturnsOneTimeKeyAndStoredHash(t *testing.T) {
	key, stored, err := GenerateClientKey("client_test")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	if key == "" || stored.Prefix == "" || stored.Hash == "" {
		t.Fatalf("generated key material is incomplete: key=%q stored=%+v", key, stored)
	}
	if !VerifyClientKey(key, stored.Hash) {
		t.Fatal("VerifyClientKey() = false, want true")
	}
	if VerifyClientKey(key+"x", stored.Hash) {
		t.Fatal("VerifyClientKey() accepted a modified key")
	}

	hash := HashClientKey(key)
	if !VerifyClientKey(key, hash) {
		t.Fatal("VerifyClientKey() rejected HashClientKey output")
	}
}

func TestVerifyClientKeyRejectsWrongKey(t *testing.T) {
	key, stored, err := GenerateClientKey("client_test")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}

	wrongKey, _, err := GenerateClientKey("client_other")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}

	if VerifyClientKey(wrongKey, stored.Hash) {
		t.Fatal("VerifyClientKey() accepted the wrong key")
	}
	if VerifyClientKey(key, "sha512:not-a-supported-hash") {
		t.Fatal("VerifyClientKey() accepted an unsupported hash version")
	}
	if VerifyClientKey(key, "not-a-valid-hash") {
		t.Fatal("VerifyClientKey() accepted a malformed hash")
	}
}

func TestClientRegistryAuthenticatesEnabledClient(t *testing.T) {
	key, stored, err := GenerateClientKey("client_test")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	registry := newClientRegistry([]Client{{
		ID:        "client_test",
		Name:      "Test Client",
		Status:    ClientStatusEnabled,
		KeyPrefix: stored.Prefix,
		KeyHash:   stored.Hash,
	}})

	client, reason := registry.authenticate(key)
	if reason != "" {
		t.Fatalf("authenticate() reason = %q, want empty", reason)
	}
	if client.ID != "client_test" || client.Name != "Test Client" {
		t.Fatalf("authenticate() client = %+v, want client_test", client)
	}
}

func TestClientRegistryRejectsDisabledAndRevokedClients(t *testing.T) {
	disabledKey, disabledStored, err := GenerateClientKey("client_disabled")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	revokedKey, revokedStored, err := GenerateClientKey("client_revoked")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	enabledKey, enabledStored, err := GenerateClientKey("client_enabled")
	if err != nil {
		t.Fatalf("GenerateClientKey() error = %v", err)
	}
	registry := newClientRegistry([]Client{
		{ID: "client_disabled", Status: ClientStatusDisabled, KeyPrefix: disabledStored.Prefix, KeyHash: disabledStored.Hash},
		{ID: "client_revoked", Status: ClientStatusRevoked, KeyPrefix: revokedStored.Prefix, KeyHash: revokedStored.Hash},
		{ID: "client_enabled", Status: ClientStatusEnabled, KeyPrefix: enabledStored.Prefix, KeyHash: enabledStored.Hash},
	})

	if _, reason := registry.authenticate(""); reason != "missing_client_key" {
		t.Fatalf("authenticate(empty) reason = %q, want missing_client_key", reason)
	}
	if _, reason := registry.authenticate(enabledKey + "x"); reason != "invalid_client_key" {
		t.Fatalf("authenticate(modified) reason = %q, want invalid_client_key", reason)
	}
	if _, reason := registry.authenticate(disabledKey); reason != "disabled_client_key" {
		t.Fatalf("authenticate(disabled) reason = %q, want disabled_client_key", reason)
	}
	if _, reason := registry.authenticate(revokedKey); reason != "revoked_client_key" {
		t.Fatalf("authenticate(revoked) reason = %q, want revoked_client_key", reason)
	}
}
