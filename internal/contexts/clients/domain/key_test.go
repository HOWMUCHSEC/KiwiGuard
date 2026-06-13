package clients

import (
	"strings"
	"testing"
)

func TestGenerateKeyReturnsRawKeyAndStoredMaterial(t *testing.T) {
	rawKey, material, err := GenerateKey("client-a")
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	if !strings.HasPrefix(rawKey, "kgc_client-a_") {
		t.Fatalf("raw key prefix = %q, want kgc_client-a_", rawKey)
	}
	if !strings.HasPrefix(material.Prefix, "kgc_client-a_") {
		t.Fatalf("material prefix = %q, want kgc_client-a_", material.Prefix)
	}
	if !strings.HasPrefix(rawKey, material.Prefix+".") {
		t.Fatalf("raw key %q does not include material prefix %q", rawKey, material.Prefix)
	}
	if !strings.HasPrefix(material.Hash, keyHashScheme) {
		t.Fatalf("hash = %q, want %q prefix", material.Hash, keyHashScheme)
	}
	if !VerifyKey(rawKey, material.Hash) {
		t.Fatal("VerifyKey(rawKey, generated hash) = false, want true")
	}
}

func TestGenerateKeyRequiresClientID(t *testing.T) {
	if _, _, err := GenerateKey(""); err == nil {
		t.Fatal("GenerateKey empty client id returned nil error")
	}
}

func TestHashKeyAndVerifyKey(t *testing.T) {
	hash := HashKey("secret")

	if !VerifyKey("secret", hash) {
		t.Fatal("VerifyKey(secret, hash) = false, want true")
	}
	if VerifyKey("wrong", hash) {
		t.Fatal("VerifyKey(wrong, hash) = true, want false")
	}
}

func TestVerifyKeyRejectsMalformedStoredHash(t *testing.T) {
	tests := []string{
		"",
		"sha512:abc",
		"sha256:not-hex",
		"sha256:abcd",
	}

	for _, storedHash := range tests {
		t.Run(storedHash, func(t *testing.T) {
			if VerifyKey("secret", storedHash) {
				t.Fatalf("VerifyKey accepted malformed hash %q", storedHash)
			}
		})
	}
}
