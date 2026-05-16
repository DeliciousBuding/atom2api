package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/atom2api/atom2api/database"
)

func GenerateAPIKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sk-atom2api-" + hex.EncodeToString(b)
}

func CreateAPIKey(db *database.DB, name string, quotaLimit int64, expiresAt *time.Time) (string, error) {
	key := GenerateAPIKey()
	_, err := db.InsertAPIKey(name, key, quotaLimit, expiresAt)
	if err != nil {
		return "", fmt.Errorf("insert api key: %w", err)
	}
	return key, nil
}
