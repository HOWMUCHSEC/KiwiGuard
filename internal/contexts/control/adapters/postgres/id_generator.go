package postgres

import (
	"crypto/rand"
	"encoding/hex"
)

// randomID generates a short random identifier with the supplied prefix.
func randomID(prefix string) (string, error) {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(bytes[:]), nil
}
