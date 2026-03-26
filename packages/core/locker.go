package pactmigrate

import (
	"context"
	"database/sql"
	"fmt"
)

// Stable advisory lock key for migrations (arbitrary int64; must fit PostgreSQL bigint).
const pgMigrationLockKey int64 = 0x504143744D677231 // "PACtMgri" as packed marker

// mySQLMigrationLockName fits MySQL GET_LOCK's 64-character name limit.
const mySQLMigrationLockName = "pactmigrate_migration_lock"

// postgresLocker uses pg_advisory_lock / pg_advisory_unlock on a session.
type postgresLocker struct {
	key int64
}

// NewPostgresLocker returns a Locker backed by pg_advisory_lock. The lock is
// held for the lifetime of the database session (connection).
func NewPostgresLocker() Locker {
	return &postgresLocker{key: pgMigrationLockKey}
}

func (l *postgresLocker) Lock(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, l.key)
	if err != nil {
		return fmt.Errorf("pactmigrate: pg_advisory_lock: %w", err)
	}
	return nil
}

func (l *postgresLocker) Unlock(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, l.key)
	if err != nil {
		return fmt.Errorf("pactmigrate: pg_advisory_unlock: %w", err)
	}
	return nil
}

// mysqlLocker uses GET_LOCK / RELEASE_LOCK for a named lock.
type mysqlLocker struct {
	name string
}

// NewMySQLLocker returns a Locker backed by GET_LOCK. name must be at most 64
// characters (MySQL limit). The default name is used if name is empty.
func NewMySQLLocker(name string) Locker {
	if name == "" {
		name = mySQLMigrationLockName
	}
	return &mysqlLocker{name: name}
}

func (l *mysqlLocker) Lock(ctx context.Context, conn *sql.Conn) error {
	var v sql.NullInt64
	// timeout -1: wait until lock is available.
	err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, ?)`, l.name, -1).Scan(&v)
	if err != nil {
		return fmt.Errorf("pactmigrate: GET_LOCK: %w", err)
	}
	if !v.Valid || v.Int64 != 1 {
		return fmt.Errorf("pactmigrate: GET_LOCK: unexpected result %v", v)
	}
	return nil
}

func (l *mysqlLocker) Unlock(ctx context.Context, conn *sql.Conn) error {
	var v sql.NullInt64
	err := conn.QueryRowContext(ctx, `SELECT RELEASE_LOCK(?)`, l.name).Scan(&v)
	if err != nil {
		return fmt.Errorf("pactmigrate: RELEASE_LOCK: %w", err)
	}
	if !v.Valid || v.Int64 != 1 {
		return fmt.Errorf("pactmigrate: RELEASE_LOCK: unexpected result %v", v)
	}
	return nil
}
