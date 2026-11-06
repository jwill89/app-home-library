package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

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
		return 0, fmt.Errorf("migrate: %w", err)
	}
	return p.GetDBVersion(ctx)
}
