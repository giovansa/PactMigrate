package pactmigrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Migration is one versioned SQL file discovered from embed.FS.
type Migration struct {
	Timestamp   string
	ContentHash string
	Title       string
	Filename    string
	SQL         []byte
}

// Key returns a stable identifier for ordering and store lookups.
func (m Migration) Key() string {
	return fmt.Sprintf("%s_%s_%s", m.Timestamp, m.ContentHash, m.Title)
}

// MigrationContentSHA256 returns the lowercase hex SHA-256 digest of the migration SQL bytes.
func MigrationContentSHA256(m Migration) string {
	sum := sha256.Sum256(m.SQL)
	return hex.EncodeToString(sum[:])
}

// VerifyContent checks that the embedded hash matches the SHA-256 of the SQL.
// Short hashes (e.g. 8 hex chars) are compared to the prefix of the full digest.
func (m Migration) VerifyContent() error {
	sum := sha256.Sum256(m.SQL)
	full := hex.EncodeToString(sum[:])
	want := strings.ToLower(strings.TrimSpace(m.ContentHash))
	if len(want) == 64 {
		if full != want {
			return fmt.Errorf("pactmigrate: content hash mismatch for %s: file %q, computed %q", m.Filename, want, full)
		}
		return nil
	}
	if len(want) < 8 {
		return fmt.Errorf("pactmigrate: content hash too short in filename %s", m.Filename)
	}
	prefixLen := min(len(want), len(full))
	if len(full) < len(want) || full[:prefixLen] != want {
		return fmt.Errorf("pactmigrate: content hash mismatch for %s: expected prefix %q, computed prefix %q", m.Filename, want, full[:prefixLen])
	}
	return nil
}
