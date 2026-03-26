package pactmigrate

import (
	"time"
)

// Dialect selects SQL semantics for the store and default advisory locker.
type Dialect int

const (
	DialectPostgres Dialect = iota
	DialectMySQL
)

// Option configures Migrator.
type Option func(*Migrator)

// WithTimeout applies a timeout to the context used for Up when the caller
// passes a context without a deadline. If the context already has a deadline,
// the earlier of the two applies.
func WithTimeout(d time.Duration) Option {
	return func(m *Migrator) {
		m.timeout = d
	}
}

// WithTableName sets the applied-migrations table name (default schema_migrations).
func WithTableName(name string) Option {
	return func(m *Migrator) {
		m.tableName = name
	}
}

// WithDialect selects PostgreSQL or MySQL behavior for the default SQL store and locker.
func WithDialect(d Dialect) Option {
	return func(m *Migrator) {
		m.dialect = d
	}
}

// WithFSDir sets the root directory inside the embed.FS to load from (default ".").
func WithFSDir(dir string) Option {
	return func(m *Migrator) {
		m.fsDir = dir
	}
}

// WithLocker sets the advisory lock implementation (overrides dialect default).
func WithLocker(l Locker) Option {
	return func(m *Migrator) {
		m.locker = l
	}
}

// WithStore sets the migration store (overrides default SQLStore built from dialect and table).
func WithStore(s MigrationStore) Option {
	return func(m *Migrator) {
		m.store = s
	}
}

// WithVerifyContent enables SHA-256 verification of file contents against the
// ContentHash segment in the filename.
func WithVerifyContent(enable bool) Option {
	return func(m *Migrator) {
		m.verifyContent = enable
	}
}

// WithOutOfOrderPolicy sets how late-merged migrations (older timestamp than
// some applied migration) are handled. OutOfOrderStrict rejects them; OutOfOrderAllowLate runs them (default).
func WithOutOfOrderPolicy(p OutOfOrderPolicy) Option {
	return func(m *Migrator) {
		m.outOfOrderPolicy = p
	}
}

// WithAllowChecksumMismatch allows Up to proceed when a stored content_sha256
// does not match the current file bytes (unsafe; use only for recovery).
func WithAllowChecksumMismatch(allow bool) Option {
	return func(m *Migrator) {
		m.allowChecksumMismatch = allow
	}
}

// WithHooks registers Hook implementations called before and after each migration in Up.
// Hooks are not invoked for Plan. Multiple hooks run in registration order.
func WithHooks(hooks ...Hook) Option {
	return func(m *Migrator) {
		m.hooks = append(m.hooks, hooks...)
	}
}
