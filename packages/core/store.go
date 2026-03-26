package pactmigrate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// SQLStore is a MigrationStore backed by a single table of applied keys.
type SQLStore struct {
	Dialect   Dialect
	TableName string
}

// NewSQLStore builds a store for the given dialect and table name. TableName is
// validated as a simple identifier ([a-zA-Z_][a-zA-Z0-9_]*).
func NewSQLStore(d Dialect, tableName string) (*SQLStore, error) {
	if err := validateIdentifier(tableName); err != nil {
		return nil, fmt.Errorf("pactmigrate: table name: %w", err)
	}
	return &SQLStore{Dialect: d, TableName: tableName}, nil
}

func validateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("empty identifier")
	}
	if len(s) > 63 {
		return fmt.Errorf("identifier longer than 63 characters")
	}
	for i, r := range s {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return fmt.Errorf("must start with letter or underscore")
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return fmt.Errorf("invalid character %q", r)
		}
	}
	return nil
}

func (s *SQLStore) quotedTable() string {
	switch s.Dialect {
	case DialectMySQL:
		return "`" + strings.ReplaceAll(s.TableName, "`", "``") + "`"
	default:
		return `"` + strings.ReplaceAll(s.TableName, `"`, `""`) + `"`
	}
}

func (s *SQLStore) Init(ctx context.Context, conn *sql.Conn) error {
	var ddl string
	switch s.Dialect {
	case DialectMySQL:
		ddl = fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (
  migration_key VARCHAR(512) NOT NULL PRIMARY KEY,
  content_sha256 VARCHAR(64) NULL,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`, s.quotedTable(),
		)
	default:
		ddl = fmt.Sprintf(
			`CREATE TABLE IF NOT EXISTS %s (
  migration_key TEXT NOT NULL PRIMARY KEY,
  content_sha256 VARCHAR(64) NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`, s.quotedTable(),
		)
	}
	_, err := conn.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("pactmigrate: create migrations table: %w", err)
	}
	// If the table already existed (e.g. from another tool) with a different
	// layout, CREATE TABLE IF NOT EXISTS does nothing. Add missing columns.
	if err := s.ensureColumns(ctx, conn); err != nil {
		return fmt.Errorf("pactmigrate: ensure migrations table columns: %w", err)
	}
	return nil
}

func (s *SQLStore) ensureColumns(ctx context.Context, conn *sql.Conn) error {
	switch s.Dialect {
	case DialectMySQL:
		return s.ensureMySQLColumns(ctx, conn)
	default:
		return s.ensurePostgresColumns(ctx, conn)
	}
}

func (s *SQLStore) ensurePostgresColumns(ctx context.Context, conn *sql.Conn) error {
	tbl := s.quotedTable()
	ok, err := s.pgColumnExists(ctx, conn, "migration_key")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN migration_key TEXT`, tbl)); err != nil {
			return fmt.Errorf("add column migration_key: %w", err)
		}
	}
	ok, err = s.pgColumnExists(ctx, conn, "applied_at")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, tbl)); err != nil {
			return fmt.Errorf("add column applied_at: %w", err)
		}
	}
	ok, err = s.pgColumnExists(ctx, conn, "content_sha256")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN content_sha256 VARCHAR(64) NULL`, tbl)); err != nil {
			return fmt.Errorf("add column content_sha256: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) pgColumnExists(ctx context.Context, conn *sql.Conn, column string) (bool, error) {
	var n int
	// Match tables visible on search_path (not only current_schema()), so we detect
	// columns on public.schema_migrations even when current_schema() is another schema.
	err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_catalog = current_database()
		  AND table_schema = ANY (current_schemas(true))
		  AND lower(table_name) = lower($1)
		  AND column_name = $2
	`, s.TableName, column).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *SQLStore) ensureMySQLColumns(ctx context.Context, conn *sql.Conn) error {
	tbl := s.quotedTable()
	ok, err := s.mySQLColumnExists(ctx, conn, "migration_key")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN migration_key VARCHAR(512) NULL`, tbl)); err != nil {
			return fmt.Errorf("add column migration_key: %w", err)
		}
	}
	ok, err = s.mySQLColumnExists(ctx, conn, "applied_at")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`, tbl)); err != nil {
			return fmt.Errorf("add column applied_at: %w", err)
		}
	}
	ok, err = s.mySQLColumnExists(ctx, conn, "content_sha256")
	if err != nil {
		return err
	}
	if !ok {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN content_sha256 VARCHAR(64) NULL`, tbl)); err != nil {
			return fmt.Errorf("add column content_sha256: %w", err)
		}
	}
	return nil
}

func (s *SQLStore) mySQLColumnExists(ctx context.Context, conn *sql.Conn, column string) (bool, error) {
	var n int
	err := conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = DATABASE()
		  AND table_name = ?
		  AND column_name = ?
	`, s.TableName, column).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *SQLStore) ListApplied(ctx context.Context, conn *sql.Conn) (map[string]AppliedRecord, error) {
	q := fmt.Sprintf(
		`SELECT migration_key, COALESCE(content_sha256, '') FROM %s WHERE migration_key IS NOT NULL AND migration_key <> ''`,
		s.quotedTable(),
	)
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: list applied: %w", err)
	}
	defer rows.Close()

	out := make(map[string]AppliedRecord)
	for rows.Next() {
		var key, sha string
		if err := rows.Scan(&key, &sha); err != nil {
			return nil, fmt.Errorf("pactmigrate: scan applied: %w", err)
		}
		out[key] = AppliedRecord{Key: key, ContentSHA256Hex: strings.ToLower(strings.TrimSpace(sha))}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pactmigrate: rows: %w", err)
	}
	return out, nil
}

func (s *SQLStore) RecordApplied(ctx context.Context, tx *sql.Tx, m Migration) error {
	sum := MigrationContentSHA256(m)
	var q string
	switch s.Dialect {
	case DialectMySQL:
		q = fmt.Sprintf(`INSERT INTO %s (migration_key, content_sha256) VALUES (?, ?)`, s.quotedTable())
	default:
		q = fmt.Sprintf(`INSERT INTO %s (migration_key, content_sha256) VALUES ($1, $2)`, s.quotedTable())
	}
	if _, err := tx.ExecContext(ctx, q, m.Key(), sum); err != nil {
		return fmt.Errorf("pactmigrate: record applied: %w", err)
	}
	return nil
}
