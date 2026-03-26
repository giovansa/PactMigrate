#!/usr/bin/env bash
# Creates apps/pactmigrate-cli/migrations/<Timestamp>_<ContentHash>_<Title>.sql
# ContentHash = first 8 hex chars of SHA-256 of the exact file bytes on disk.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
MIGRATIONS_DIR="${ROOT}/apps/pactmigrate-cli/migrations"

raw="${1:-}"
if [[ -z "${raw// }" ]]; then
	echo "usage: $0 <title>" >&2
	echo "  Example: $0 add_users_table" >&2
	echo "  Or:      make migration name=add_users_table" >&2
	exit 1
fi

slug=$(printf '%s' "$raw" | tr '[:upper:]' '[:lower:]' | tr ' ' '_' | tr -cd 'a-z0-9_')
if [[ -z "$slug" ]]; then
	echo "$0: title becomes empty after sanitizing; use letters, numbers, spaces, or underscores" >&2
	exit 1
fi

ts=$(date +%Y%m%d%H%M)

body="-- PactMigrate migration: ${slug}
-- Edit the SQL below. After you change this file, run: make migration-rehash FILE=path/to/this/file.sql
-- (ContentHash in the filename must match SHA-256 of the file for WithVerifyContent(true).)

"

mkdir -p "$MIGRATIONS_DIR"

# Single source of truth: hash the bytes we are about to commit as the file.
tmp=$(mktemp "${MIGRATIONS_DIR}/.new-migration.XXXXXX")
trap 'rm -f "$tmp"' EXIT
printf '%s' "$body" >"$tmp"

hash=$(openssl dgst -sha256 "$tmp" 2>/dev/null | awk '{print $NF}' | cut -c1-8)
out="${MIGRATIONS_DIR}/${ts}_${hash}_${slug}.sql"

if [[ -e "$out" ]]; then
	echo "$0: refusing to overwrite existing file: $out" >&2
	exit 1
fi

mv "$tmp" "$out"
trap - EXIT

# Verify filename matches on-disk content (detects mktemp / mv issues).
actual=$(openssl dgst -sha256 "$out" | awk '{print $NF}' | cut -c1-8)
if [[ "$actual" != "$hash" ]]; then
	echo "$0: internal error: hash mismatch ($hash vs $actual) for $out" >&2
	exit 1
fi

echo "Created $out"
