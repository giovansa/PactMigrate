package pactmigrate

import "context"

// Hook is called around each migration execution in Up (not in Plan).
type Hook interface {
	BeforeMigration(ctx context.Context, m Migration) error
	AfterMigration(ctx context.Context, m Migration, err error) error
}

type hookChain []Hook

func (c hookChain) before(ctx context.Context, m Migration) error {
	for _, h := range c {
		if err := h.BeforeMigration(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

func (c hookChain) after(ctx context.Context, m Migration, err error) error {
	for _, h := range c {
		if err2 := h.AfterMigration(ctx, m, err); err2 != nil {
			return err2
		}
	}
	return nil
}
