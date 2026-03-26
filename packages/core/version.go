package pactmigrate

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// ParseFilename parses a hybrid version name: {Timestamp}_{ContentHash}_{Title}.sql
// Timestamp must be one or more ASCII digits (typically YYYYMMDDHHmm or YYYYMMDDHHmmss).
// ContentHash must be hexadecimal (length 8–64, inclusive), representing a prefix or full SHA-256 digest.
// Title may contain underscores; it is everything after the second underscore, before ".sql".
func ParseFilename(name string) (timestamp, contentHash, title string, err error) {
	base := path.Base(name)
	if !strings.HasSuffix(strings.ToLower(base), ".sql") {
		return "", "", "", fmt.Errorf("pactmigrate: %q: expected .sql suffix", name)
	}
	stem := strings.TrimSuffix(base, path.Ext(base))
	if stem == "" {
		return "", "", "", fmt.Errorf("pactmigrate: %q: empty filename", name)
	}

	parts := strings.SplitN(stem, "_", 3)
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("pactmigrate: %q: need Timestamp_Hash_Title with at least two underscores", name)
	}

	ts, hash, rest := parts[0], parts[1], parts[2]
	if ts == "" || !isAllASCIIDigits(ts) {
		return "", "", "", fmt.Errorf("pactmigrate: %q: invalid timestamp %q", name, ts)
	}
	if hash == "" || !isHexString(hash) {
		return "", "", "", fmt.Errorf("pactmigrate: %q: invalid content hash %q", name, hash)
	}
	hl := len(hash)
	if hl < 8 || hl > 64 {
		return "", "", "", fmt.Errorf("pactmigrate: %q: content hash length must be 8-64 hex chars, got %d", name, hl)
	}
	if rest == "" {
		return "", "", "", fmt.Errorf("pactmigrate: %q: title is empty", name)
	}

	return ts, strings.ToLower(hash), rest, nil
}

func isAllASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}
