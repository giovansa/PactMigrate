package pactmigrate

import (
	"context"
	"database/sql"
)

// Locker acquires a database session–level advisory lock so only one migration
// process runs at a time. Implementations must use the same *sql.Conn that
// executes migrations so the lock is tied to that session (released when the
// connection is closed or the process exits).
type Locker interface {
	Lock(ctx context.Context, conn *sql.Conn) error
	Unlock(ctx context.Context, conn *sql.Conn) error
}

// MigrationStore persists which migrations have been applied. Init and
// ListApplied use the same *sql.Conn as migrations. RecordApplied runs inside
// the migration transaction so DDL and bookkeeping commit or roll back
// together.
type MigrationStore interface {
	Init(ctx context.Context, conn *sql.Conn) error
	ListApplied(ctx context.Context, conn *sql.Conn) (map[string]AppliedRecord, error)
	RecordApplied(ctx context.Context, tx *sql.Tx, m Migration) error
}
