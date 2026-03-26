package pactmigrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"
)

// Migrator runs versioned SQL migrations from an io/fs.FS (including embed.FS).
type Migrator struct {
	db *sql.DB
	fs fs.FS

	fsDir     string
	dialect   Dialect
	locker    Locker
	store     MigrationStore
	tableName string

	timeout       time.Duration
	verifyContent bool

	hooks                 hookChain
	outOfOrderPolicy      OutOfOrderPolicy
	allowChecksumMismatch bool
}

// New constructs a Migrator. The filesystem must contain .sql files named with
// the hybrid Timestamp_Hash_Title convention. Defaults: PostgreSQL dialect,
// table schema_migrations, session advisory lock, FS root ".", out-of-order allow-late.
func New(db *sql.DB, filesystem fs.FS, opts ...Option) (*Migrator, error) {
	if db == nil {
		return nil, fmt.Errorf("pactmigrate: database is nil")
	}
	if filesystem == nil {
		return nil, fmt.Errorf("pactmigrate: filesystem is nil")
	}

	m := &Migrator{
		db:               db,
		fs:               filesystem,
		fsDir:            ".",
		dialect:          DialectPostgres,
		tableName:        "schema_migrations",
		outOfOrderPolicy: OutOfOrderAllowLate,
	}
	for _, o := range opts {
		o(m)
	}

	if m.locker == nil {
		switch m.dialect {
		case DialectMySQL:
			m.locker = NewMySQLLocker("")
		default:
			m.locker = NewPostgresLocker()
		}
	}
	if m.store == nil {
		st, err := NewSQLStore(m.dialect, m.tableName)
		if err != nil {
			return nil, fmt.Errorf("pactmigrate: migration store: %w", err)
		}
		m.store = st
	}

	return m, nil
}

// Up applies pending migrations in order. It uses one database connection for
// advisory locking and runs each migration in its own transaction (DDL + store
// insert) so failures roll back cleanly.
func (m *Migrator) Up(ctx context.Context) (err error) {
	if m.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.timeout)
		defer cancel()
	}

	conn, err := m.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pactmigrate: acquire connection: %w", err)
	}
	defer conn.Close()

	locked := false
	defer func() {
		if !locked {
			return
		}
		uctx := context.Background()
		if m.timeout > 0 {
			var cancel context.CancelFunc
			uctx, cancel = context.WithTimeout(context.Background(), m.timeout)
			defer cancel()
		}
		uerr := m.locker.Unlock(uctx, conn)
		if uerr == nil {
			return
		}
		uerr = fmt.Errorf("pactmigrate: advisory unlock: %w", uerr)
		if err != nil {
			err = errors.Join(err, uerr)
			return
		}
		err = uerr
	}()

	if err := m.locker.Lock(ctx, conn); err != nil {
		return fmt.Errorf("pactmigrate: advisory lock: %w", err)
	}
	locked = true

	if err := m.store.Init(ctx, conn); err != nil {
		return fmt.Errorf("pactmigrate: migration store init: %w", err)
	}

	applied, err := m.store.ListApplied(ctx, conn)
	if err != nil {
		return fmt.Errorf("pactmigrate: list applied: %w", err)
	}

	migrations, err := Load(m.fs, m.fsDir)
	if err != nil {
		return fmt.Errorf("pactmigrate: load migrations: %w", err)
	}

	if err := m.verifyAppliedChecksums(applied, migrations); err != nil {
		return err
	}

	pending := m.collectPending(applied, migrations)
	if err := m.checkOutOfOrder(applied, pending); err != nil {
		return err
	}

	for _, mig := range pending {
		if m.verifyContent {
			if verr := mig.VerifyContent(); verr != nil {
				return fmt.Errorf("pactmigrate: %w", verr)
			}
		}
		if err := m.applyOne(ctx, conn, mig); err != nil {
			return err
		}
	}

	return nil
}

func (m *Migrator) verifyAppliedChecksums(applied map[string]AppliedRecord, migrations []Migration) error {
	byKey := make(map[string]Migration, len(migrations))
	for _, mig := range migrations {
		byKey[mig.Key()] = mig
	}
	for key := range applied {
		if _, ok := byKey[key]; !ok {
			return fmt.Errorf("pactmigrate: migration store lists applied key %q but no matching .sql file was loaded", key)
		}
	}
	for key, rec := range applied {
		mig := byKey[key]
		if rec.ContentSHA256Hex == "" {
			continue
		}
		current := MigrationContentSHA256(mig)
		if strings.EqualFold(current, rec.ContentSHA256Hex) {
			continue
		}
		if m.allowChecksumMismatch {
			continue
		}
		return fmt.Errorf("pactmigrate: migration file changed after apply: %s (stored %s, current %s)", mig.Filename, rec.ContentSHA256Hex, current)
	}
	return nil
}

func (m *Migrator) collectPending(applied map[string]AppliedRecord, migrations []Migration) []Migration {
	var out []Migration
	for _, mig := range migrations {
		if _, ok := applied[mig.Key()]; !ok {
			out = append(out, mig)
		}
	}
	return out
}

func (m *Migrator) checkOutOfOrder(applied map[string]AppliedRecord, pending []Migration) error {
	if m.outOfOrderPolicy != OutOfOrderStrict {
		return nil
	}
	max := maxAppliedTimestamp(applied)
	for _, mig := range pending {
		if max != "" && mig.Timestamp < max {
			return fmt.Errorf("pactmigrate: strict out-of-order policy: pending migration %s (timestamp %s) is older than already applied migrations (max timestamp %s)", mig.Key(), mig.Timestamp, max)
		}
	}
	return nil
}

func (m *Migrator) applyOne(ctx context.Context, conn *sql.Conn, mig Migration) (err error) {
	if len(m.hooks) > 0 {
		if herr := m.hooks.before(ctx, mig); herr != nil {
			return herr
		}
		defer func() {
			if err2 := m.hooks.after(ctx, mig, err); err2 != nil {
				if err == nil {
					err = err2
				} else {
					err = errors.Join(err, err2)
				}
			}
		}()
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("pactmigrate: begin transaction for %s: %w", mig.Filename, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(mig.SQL)); err != nil {
		return fmt.Errorf("pactmigrate: execute %s: %w", mig.Filename, err)
	}
	if err := m.store.RecordApplied(ctx, tx, mig); err != nil {
		return fmt.Errorf("pactmigrate: record applied %s: %w", mig.Filename, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("pactmigrate: commit %s: %w", mig.Filename, err)
	}
	return nil
}
