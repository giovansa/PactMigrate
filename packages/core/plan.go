package pactmigrate

import (
	"context"
	"fmt"
)

// Plan is the result of Plan(): migrations that would run on Up without executing them.
type Plan struct {
	Pending []Migration
}

// Plan lists pending migrations (same ordering and filters as Up) without acquiring
// the advisory lock, executing SQL, or invoking hooks. It still runs Init on the store
// and validates stored checksums for already-applied migrations.
func (m *Migrator) Plan(ctx context.Context) (*Plan, error) {
	if m.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.timeout)
		defer cancel()
	}

	conn, err := m.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: acquire connection: %w", err)
	}
	defer conn.Close()

	if err := m.store.Init(ctx, conn); err != nil {
		return nil, fmt.Errorf("pactmigrate: migration store init: %w", err)
	}

	applied, err := m.store.ListApplied(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: list applied: %w", err)
	}

	migrations, err := Load(m.fs, m.fsDir)
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: load migrations: %w", err)
	}

	if err := m.verifyAppliedChecksums(applied, migrations); err != nil {
		return nil, err
	}

	pending := m.collectPending(applied, migrations)
	if err := m.checkOutOfOrder(applied, pending); err != nil {
		return nil, err
	}

	return &Plan{Pending: pending}, nil
}
