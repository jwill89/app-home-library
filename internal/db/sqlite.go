package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // the driver, which registers itself as "sqlite"
)

// readConns is the read pool's size, kept open: the default keeps two idle, and every
// reopened connection runs the pragmas again.
const readConns = 8

// DB holds two pools over one SQLite file: writes serialize through a single
// connection, reads use a separate pool. Every multi-statement write runs in a
// transaction on Write.
type DB struct {
	Write *sql.DB
	Read  *sql.DB
}

// Open opens the database file with the pragmas the app relies on, creating it
// and its directory if needed.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("database dir: %w", err)
	}
	dsn := "file:" + path + "?" + url.Values{
		"_pragma": {"busy_timeout(5000)", "journal_mode(WAL)", "foreign_keys(1)", "synchronous(NORMAL)"},
	}.Encode()

	// A write transaction takes the lock as it begins, so it waits out the busy timeout for
	// another process's (the text service's, the CLI's). Begun deferred, one that reads first
	// would fail at once on finding the lock taken.
	write, err := sql.Open("sqlite", dsn+"&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("open write pool: %w", err)
	}
	write.SetMaxOpenConns(1)

	read, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open read pool: %w", err), write.Close())
	}
	read.SetMaxOpenConns(readConns)
	read.SetMaxIdleConns(readConns)
	if err := write.PingContext(context.Background()); err != nil {
		return nil, errors.Join(fmt.Errorf("open %s: %w", path, err), write.Close(), read.Close())
	}
	return &DB{Write: write, Read: read}, nil
}

// InTx runs fn in a write transaction: every write of more than one statement goes through it.
func (d *DB) InTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := d.Write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(New(tx)); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

// Close closes both pools.
func (d *DB) Close() error {
	rerr := d.Read.Close()
	if err := d.Write.Close(); err != nil {
		return err
	}
	return rerr
}
