package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func provider(write *sql.DB) (*goose.Provider, error) {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectSQLite3, write, files)
}

// Migrate applies pending migrations on the write pool and returns the schema version.
func Migrate(ctx context.Context, write *sql.DB) (int64, error) {
	p, err := provider(write)
	if err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return 0, errors.Join(fmt.Errorf("migrate: %w", err), settle(ctx, write))
	}
	return p.GetDBVersion(ctx)
}

// settle puts the write connection back after a failed migration: any transaction a
// rebuild began is rolled back, and foreign keys and the usual ALTER TABLE are on again.
func settle(ctx context.Context, write *sql.DB) error {
	ctx = context.WithoutCancel(ctx)
	conn, err := write.Conn(ctx)
	if err != nil {
		return fmt.Errorf("after the failed migration: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil && !strings.Contains(err.Error(), "no transaction is active") {
		return errors.Join(fmt.Errorf("roll back the failed migration: %w", err), conn.Close())
	}
	_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys = ON; PRAGMA legacy_alter_table = OFF")
	return errors.Join(err, conn.Close())
}
