package pactmigrate

import (
	"strings"
)

// AppliedRecord is one row from the migration store (applied migration).
type AppliedRecord struct {
	Key              string
	ContentSHA256Hex string // 64 lowercase hex chars; empty if not stored (legacy rows).
}

// OutOfOrderPolicy controls pending migrations whose timestamp is older than
// migrations already applied (late merge from another branch).
type OutOfOrderPolicy int

const (
	// OutOfOrderAllowLate runs older pending migrations in sort order (default).
	OutOfOrderAllowLate OutOfOrderPolicy = iota
	// OutOfOrderStrict fails if any pending migration has a timestamp lexicographically
	// smaller than the largest timestamp among already-applied keys.
	OutOfOrderStrict
)

// timestampFromMigrationKey returns the Timestamp segment from Key() (first underscore-separated field).
func timestampFromMigrationKey(key string) string {
	parts := strings.SplitN(key, "_", 3)
	if len(parts) < 1 {
		return ""
	}
	return parts[0]
}

func maxAppliedTimestamp(applied map[string]AppliedRecord) string {
	max := ""
	for k := range applied {
		ts := timestampFromMigrationKey(k)
		if ts > max {
			max = ts
		}
	}
	return max
}
