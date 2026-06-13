package openai

import (
	"strings"

	appgateway "github.com/howmuchsec/kiwiguard/internal/contexts/gateway/application"
	"github.com/howmuchsec/kiwiguard/internal/contexts/clients/domain"
)

// GenerateClientKey returns a one-time raw key and the key material to store.
func GenerateClientKey(clientID string) (string, ClientKeyMaterial, error) {
	return clients.GenerateKey(clientID)
}

// HashClientKey returns a versioned SHA-256 hash for a raw client key.
func HashClientKey(key string) string {
	return clients.HashKey(key)
}

// VerifyClientKey reports whether key matches storedHash using constant-time comparison.
func VerifyClientKey(key, storedHash string) bool {
	return clients.VerifyKey(key, storedHash)
}

type clientRegistry struct {
	clientsByPrefix map[string]Client
}

func newClientRegistry(clients []Client) clientRegistry {
	clientsByPrefix := make(map[string]Client, len(clients))
	for _, client := range clients {
		if client.KeyPrefix == "" {
			continue
		}
		clientsByPrefix[client.KeyPrefix] = client
	}

	return clientRegistry{clientsByPrefix: clientsByPrefix}
}

func (r clientRegistry) authenticate(key string) (Client, string) {
	if key == "" {
		return Client{}, "missing_client_key"
	}

	separator := strings.LastIndex(key, ".")
	if separator < 0 {
		return Client{}, "invalid_client_key"
	}
	prefix := key[:separator]

	client, ok := r.clientsByPrefix[prefix]
	if !ok || !VerifyClientKey(key, client.KeyHash) {
		return Client{}, "invalid_client_key"
	}

	switch client.Status {
	case ClientStatusEnabled:
		return client, ""
	case ClientStatusDisabled:
		return Client{}, "disabled_client_key"
	case ClientStatusRevoked:
		return Client{}, "revoked_client_key"
	default:
		return Client{}, "disabled_client_key"
	}
}

func (r clientRegistry) AuthenticateClient(key string) (appgateway.ClientIdentity, appgateway.RejectReason) {
	client, reason := r.authenticate(key)
	if reason != "" {
		return appgateway.ClientIdentity{}, appgateway.RejectReason(reason)
	}
	return appgateway.ClientIdentity{ID: client.ID}, ""
}
