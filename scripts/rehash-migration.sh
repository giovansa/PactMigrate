#!/usr/bin/env bash
# Renames migration file(s) so the ContentHash segment matches SHA-256(file contents).
# Basename: <Timestamp>_<OldHash>_<Title>.sql (title may contain underscores).
#
# Usage:
#   scripts/rehash-migration.sh [path/to/one.sql ...]
# With no arguments, rehashes every *.sql under apps/pactmigrate-cli/migrations/.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DEFAULT_DIR="${ROOT}/apps/pactmigrate-cli/migrations"

rehash_one() {
	local f="$1"
	if [[ ! -f "$f" ]]; then
		echo "$0: not a file: $f" >&2
		return 1
	fi

	local base
	base=$(basename "$f")
	case "$base" in
	*.sql) ;;
	*)
		echo "$0: expected .sql file: $f" >&2
		return 1
		;;
	esac

	local stem ts oldhash title
	stem="${base%.sql}"
	ts=$(echo "$stem" | cut -d'_' -f1)
	oldhash=$(echo "$stem" | cut -d'_' -f2)
	title=$(echo "$stem" | cut -d'_' -f3-)

	if [[ -z "$ts" || -z "$oldhash" || -z "$title" ]]; then
		echo "$0: cannot parse Timestamp_Hash_Title from: $base" >&2
		return 1
	fi

	local newhash
	newhash=$(openssl dgst -sha256 "$f" | awk '{print $NF}' | cut -c1-8)

	if [[ "$newhash" == "$oldhash" ]]; then
		echo "Content hash already aligned: $newhash ($f)"
		return 0
	fi

	local dir newpath
	dir=$(cd "$(dirname "$f")" && pwd)
	newpath="${dir}/${ts}_${newhash}_${title}.sql"

	if [[ -e "$newpath" ]]; then
		echo "$0: refuse to overwrite existing: $newpath" >&2
		return 1
	fi

	mv "$f" "$newpath"
	echo "Renamed to $newpath (hash $oldhash -> $newhash)"
}

if [[ $# -eq 0 ]]; then
	shopt -s nullglob
	files=("$DEFAULT_DIR"/*.sql)
	if [[ ${#files[@]} -eq 0 ]]; then
		echo "$0: no .sql files in $DEFAULT_DIR" >&2
		exit 1
	fi
	for f in "${files[@]}"; do
		rehash_one "$f"
	done
	exit 0
fi

for f in "$@"; do
	rehash_one "$f"
done
