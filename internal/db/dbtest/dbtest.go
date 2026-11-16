// Package dbtest gives each test a database of its own, migrated: a copy of one template the
// test binary migrates once, in memory.
package dbtest

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"

	"homelibrary/internal/db"
)

// template is the test binary's one migrated database, kept open in memory for its whole run:
// a shared-cache memory database lasts while a connection to it does.
type template struct {
	once sync.Once
	conn *sql.DB
	err  error
}

func (tm *template) open() (*sql.DB, error) {
	tm.once.Do(func() {
		tm.conn, tm.err = sql.Open("sqlite", "file:dbtest-template?mode=memory&cache=shared&_pragma=foreign_keys(1)")
		if tm.err != nil {
			return
		}
		tm.conn.SetMaxIdleConns(1)
		tm.conn.SetConnMaxLifetime(0)
		_, tm.err = db.Migrate(context.Background(), tm.conn)
	})
	return tm.conn, tm.err
}

// migrated is the template every Open copies: the one value this package keeps, since it is
// built once for every test in the binary.
var migrated template

// Path copies the migrated template into the test's temp folder and returns the copy's path,
// for a test that opens it the way the app does.
func Path(t testing.TB) string {
	t.Helper()
	tmpl, err := migrated.open()
	if err != nil {
		t.Fatalf("migrate the template database: %v", err)
	}
	path := filepath.Join(t.TempDir(), "test.sqlite")
	if _, err := tmpl.ExecContext(context.Background(), "VACUUM INTO ?", path); err != nil {
		t.Fatalf("copy the template database: %v", err)
	}
	return path
}

// Open copies the migrated template and opens the copy as the app does, closing it when the
// test ends.
func Open(t testing.TB) *db.DB {
	t.Helper()
	database, err := db.Open(Path(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
