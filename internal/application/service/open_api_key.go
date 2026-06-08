package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const openAPIKeyPrefix = "sk-open-"

// HashOpenAPIKey returns the SHA-256 hex digest of a partner API key.
func HashOpenAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

// GenerateOpenAPIKey creates a new partner credential and its stored hash.
func GenerateOpenAPIKey() (plaintext string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate open api key: %w", err)
	}
	plaintext = openAPIKeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	hash = HashOpenAPIKey(plaintext)
	return plaintext, hash, nil
}

// BuildOpenAPIInternalUserID returns a stable shadow user id (UUID) for a partner user.
// Must fit sessions.user_id VARCHAR(36).
func BuildOpenAPIInternalUserID(clientID, externalUserID string) string {
	clientID = strings.TrimSpace(clientID)
	externalUserID = strings.TrimSpace(externalUserID)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("open-api:%s:%s", clientID, externalUserID))).String()
}

func isKBAllowed(allowed []string, kbID string) bool {
	for _, id := range allowed {
		if id == kbID {
			return true
		}
	}
	return false
}
