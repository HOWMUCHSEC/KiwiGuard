package clients

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	keyHashScheme = "sha256:"
	keyPrefixSize = 8
	keySecretSize = 32
)

// GenerateKey returns a one-time raw client key and the material to store.
func GenerateKey(clientID string) (string, KeyMaterial, error) {
	if clientID == "" {
		return "", KeyMaterial{}, fmt.Errorf("client id is required")
	}

	prefixBytes, err := randomBytes(keyPrefixSize)
	if err != nil {
		return "", KeyMaterial{}, fmt.Errorf("generate key prefix: %w", err)
	}
	secretBytes, err := randomBytes(keySecretSize)
	if err != nil {
		return "", KeyMaterial{}, fmt.Errorf("generate key secret: %w", err)
	}

	prefix := "kgc_" + clientID + "_" + base64.RawURLEncoding.EncodeToString(prefixBytes)
	key := prefix + "." + base64.RawURLEncoding.EncodeToString(secretBytes)

	return key, KeyMaterial{
		Prefix: prefix,
		Hash:   HashKey(key),
	}, nil
}

// HashKey returns a versioned SHA-256 hash for a raw client key.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return keyHashScheme + hex.EncodeToString(sum[:])
}

// VerifyKey reports whether key matches storedHash using constant-time comparison.
func VerifyKey(key, storedHash string) bool {
	hashHex, ok := strings.CutPrefix(storedHash, keyHashScheme)
	if !ok {
		return false
	}

	stored, err := hex.DecodeString(hashHex)
	if err != nil || len(stored) != sha256.Size {
		return false
	}

	sum := sha256.Sum256([]byte(key))
	return subtle.ConstantTimeCompare(sum[:], stored) == 1
}

func randomBytes(size int) ([]byte, error) {
	out := make([]byte, size)
	if _, err := rand.Read(out); err != nil {
		return nil, err
	}
	return out, nil
}
