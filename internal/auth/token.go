package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// MakeRefreshToken generates a random 256-bit (32-byte) hex-encoded string
func MakeRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
