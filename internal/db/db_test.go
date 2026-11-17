package db

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"homelibrary/internal/civil"
)

var update = flag.Bool("update", false, "write schema.sql from what the migrations build")

// empty is a new database as the owner makes one: an empty file, opened, not yet migrated.
func empty(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func migrated(t *testing.T) *DB {
	t.Helper()
	d := empty(t)
	if _, err := Migrate(context.Background(), d.Write); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestOpenRefusesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sqlite")
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "no database at") {
		t.Fatalf("opened a missing file: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open made the file: %v", err)
	}
}

func TestPragmas(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	for name, pool := range map[string]*sql.DB{"write": d.Write, "read": d.Read} {
		for pragma, want := range map[string]string{
			"journal_mode": "wal", "foreign_keys": "1", "busy_timeout": "5000", "synchronous": "1",
		} {
			var got string
			if err := pool.QueryRowContext(ctx, "PRAGMA "+pragma).Scan(&got); err != nil || got != want {
				t.Errorf("%s pool: %s = %q, want %q (err %v)", name, pragma, got, want, err)
			}
		}
	}
}

// schemaOf is what the migrations built, one statement each, in an order sqlc can read: the
// tables, then the views, each after any view it reads, then the indexes and the triggers.
// goose's own table and the full-text index's shadow tables are left out.
func schemaOf(t *testing.T, d *DB) string {
	t.Helper()
	rows, err := d.Read.QueryContext(context.Background(), `
		SELECT type, name, sql FROM sqlite_master m
		WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite\_%' ESCAPE '\' AND name <> 'goose_db_version'
			AND NOT EXISTS (
				SELECT 1 FROM sqlite_master v
				WHERE v.sql LIKE 'CREATE VIRTUAL TABLE%' AND m.name LIKE v.name || '\_%' ESCAPE '\'
			)
		ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'view' THEN 1 WHEN 'index' THEN 2 ELSE 3 END, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	type object struct{ kind, name, sql string }
	var objects, views []object
	for rows.Next() {
		var o object
		if err := rows.Scan(&o.kind, &o.name, &o.sql); err != nil {
			t.Fatal(err)
		}
		if o.kind == "view" {
			views = append(views, o)
			continue
		}
		objects = append(objects, o)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	// Each view goes once every view it names has gone.
	var ordered []object
	placed := map[string]bool{}
	for len(ordered) < len(views) {
		before := len(ordered)
		for _, v := range views {
			if placed[v.name] {
				continue
			}
			ready := true
			for _, other := range views {
				if other.name != v.name && !placed[other.name] && regexp.MustCompile(`\b`+other.name+`\b`).MatchString(v.sql) {
					ready = false
				}
			}
			if ready {
				ordered = append(ordered, v)
				placed[v.name] = true
			}
		}
		if len(ordered) == before {
			t.Fatal("the views read each other in a circle")
		}
	}
	tables := 0
	for tables < len(objects) && objects[tables].kind == "table" {
		tables++
	}
	objects = append(append(append([]object{}, objects[:tables]...), ordered...), objects[tables:]...)
	var b strings.Builder
	b.WriteString("-- The schema the migrations build, written by TestSchema (task gen) and never by hand.\n")
	for _, o := range objects {
		_, _ = fmt.Fprintf(&b, "\n%s;\n", o.sql)
	}
	return b.String()
}

// TestSchema holds schema.sql to what the migrations build, so a schema change shows in review;
// sqlc reads the queries' types from it. `go test ./internal/db -run TestSchema -update` writes it.
func TestSchema(t *testing.T) {
	got := schemaOf(t, migrated(t))
	if *update {
		if err := os.WriteFile("schema.sql", []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ReplaceAll(string(want), "\r\n", "\n") != got {
		t.Error("schema.sql is not what the migrations build: run task gen")
	}
}

func TestUserVersionIsGooseVersion(t *testing.T) {
	d := empty(t)
	ctx := context.Background()
	v, err := Migrate(ctx, d.Write)
	if err != nil {
		t.Fatal(err)
	}
	var user int64
	if err := d.Write.QueryRowContext(ctx, "PRAGMA user_version").Scan(&user); err != nil {
		t.Fatal(err)
	}
	if user != v {
		t.Errorf("user_version %d, goose's version %d", user, v)
	}
}

// TestTimes holds what the code relies on the driver and the CHECKs for: a timestamp read back
// in the stored form, as a time.Time or as text; a day read through Date; and every other form
// refused.
func TestTimes(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	if _, err := d.Write.ExecContext(ctx, `INSERT INTO users (name, created_at) VALUES ('Owner', '2026-11-07T09:18:09Z')`); err != nil {
		t.Fatal(err)
	}
	var at time.Time
	if err := d.Read.QueryRowContext(ctx, "SELECT created_at FROM users").Scan(&at); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 7, 9, 18, 9, 0, time.UTC); !at.Equal(want) || at.Location() != time.UTC {
		t.Errorf("as a time: %v", at)
	}
	var text string
	if err := d.Read.QueryRowContext(ctx, "SELECT created_at FROM users").Scan(&text); err != nil || text != "2026-11-07T09:18:09Z" {
		t.Errorf("as text: %q, err %v", text, err)
	}

	for _, bad := range []any{
		"2026-11-07 09:18:09", "2026-11-07T09:18:09+01:00", "2026-11-07T09:18:09.401Z",
		"2026-02-30T09:18:09Z", "", time.Date(2026, 11, 7, 9, 18, 9, 0, time.UTC),
	} {
		if _, err := d.Write.ExecContext(ctx, `INSERT INTO users (name, created_at) VALUES ('Bad', ?)`, bad); err == nil {
			t.Errorf("a timestamp of %q was taken", bad)
		}
	}

	if _, err := d.Write.ExecContext(ctx, `INSERT INTO series (name, dismissed_on) VALUES ('Set', ?), ('Unset', ?)`,
		civil.Date("2026-11-07"), civil.Date("")); err != nil {
		t.Fatal(err)
	}
	rows, err := d.Read.QueryContext(ctx, "SELECT dismissed_on, typeof(dismissed_on) FROM series ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var day civil.Date
		var kind string
		if err := rows.Scan(&day, &kind); err != nil {
			t.Fatal(err)
		}
		got = append(got, string(day)+"/"+kind)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	if strings.Join(got, " ") != "2026-11-07/text /null" {
		t.Errorf("days read back %v", got)
	}
	for _, bad := range []string{"2026-11-7", "2026-02-30", "2026-11-07T00:00:00Z", ""} {
		if _, err := d.Write.ExecContext(ctx, `INSERT INTO series (name, dismissed_on) VALUES (?, ?)`, "Bad "+bad, bad); err == nil {
			t.Errorf("a day of %q was taken", bad)
		}
	}
	if _, err := civil.Date("2026-02-30").Value(); err == nil {
		t.Error("Date wrote a day that isn't one")
	}
}

// TestMigration38 moves a library with every kind of row through the rebuild: each row kept,
// each ” a NULL, each counter where it was, and back again by the way down.
func TestMigration38(t *testing.T) {
	d := empty(t)
	ctx := context.Background()
	p, err := provider(d.Write)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 37); err != nil {
		t.Fatal(err)
	}
	seed := []string{
		`INSERT INTO users (name) VALUES ('Owner')`,
		`INSERT INTO passkeys (user_id, rp_id, credential_id, credential, name, last_used_at) VALUES (1, 'localhost', 'c1', '{}', 'Phone', NULL)`,
		`INSERT INTO series (name, dismissed_on) VALUES ('Kept', ''), ('Dismissed', '2026-10-01')`,
		`INSERT INTO works (title, series_id, started_on, finished_on) VALUES ('Dune', 1, '2026-01-02', '')`,
		`INSERT INTO editions (work_id, title, wanted_on) VALUES (1, 'Dune', '')`,
		`INSERT INTO copies (edition_id, acquired_on) VALUES (1, '2025-12-25'), (1, ''), (1, '')`,
		`DELETE FROM copies WHERE id = 3`,
		`INSERT INTO shares (name, token_hash, expires_at, last_used_at) VALUES ('Never', 't1', '', '')`,
		`INSERT INTO borrowers (name) VALUES ('Sam')`,
		`INSERT INTO loans (copy_id, borrower_id, lent_on, due_on, returned_on) VALUES (1, 1, '2026-11-01', '', ''), (2, 1, '2026-10-01', '2026-10-15', '2026-10-14')`,
		`UPDATE layout_settings SET applied_at = ''`,
	}
	for _, s := range seed {
		if _, err := d.Write.ExecContext(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	var changes, lastChange int64
	if err := d.Write.QueryRowContext(ctx, "SELECT count(*), max(id) FROM changes").Scan(&changes, &lastChange); err != nil {
		t.Fatal(err)
	}
	count := func(q string) int64 {
		t.Helper()
		var n int64
		if err := d.Write.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}

	if _, err := p.UpTo(ctx, 38); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int64{
		"SELECT count(*) FROM copies":                                   2,
		"SELECT seq FROM sqlite_sequence WHERE name = 'copies'":         3,
		"SELECT count(*) FROM loans WHERE returned_on IS NULL":          1,
		"SELECT count(*) FROM loans WHERE due_on = '2026-10-15'":        1,
		"SELECT count(*) FROM series WHERE dismissed_on IS NULL":        1,
		"SELECT count(*) FROM works WHERE finished_on IS NULL":          1,
		"SELECT count(*) FROM shares WHERE expires_at IS NULL":          1,
		"SELECT count(*) FROM layout_settings WHERE applied_at IS NULL": 1,
		"SELECT count(*) FROM changes":                                  changes,
		"SELECT count(*) FROM pragma_foreign_key_check":                 0,
		"PRAGMA user_version":                                           38,
	} {
		if got := count(q); got != want {
			t.Errorf("after: %s = %d, want %d", q, got, want)
		}
	}
	// The triggers came back with their tables: a change is still recorded, after the last.
	if _, err := d.Write.ExecContext(ctx, "UPDATE loans SET notes = 'x' WHERE id = 1"); err != nil {
		t.Fatal(err)
	}
	if got := count("SELECT max(id) FROM changes"); got <= lastChange {
		t.Errorf("no change recorded after the rebuild: %d", got)
	}
	// A second loan of a copy that's out is still refused.
	if _, err := d.Write.ExecContext(ctx, "INSERT INTO loans (copy_id, borrower_id, lent_on) VALUES (1, 1, '2026-11-02')"); err == nil {
		t.Error("a copy that's out was lent again")
	}

	if _, err := p.DownTo(ctx, 37); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int64{
		"SELECT count(*) FROM loans WHERE returned_on = ''":     1,
		"SELECT count(*) FROM series WHERE dismissed_on = ''":   1,
		"SELECT seq FROM sqlite_sequence WHERE name = 'copies'": 3,
		"SELECT count(*) FROM pragma_foreign_key_check":         0,
	} {
		if got := count(q); got != want {
			t.Errorf("down: %s = %d, want %d", q, got, want)
		}
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// TestSettleAfterAFailedMigration: a rebuild that stops part way leaves its transaction open
// and foreign keys off on the write connection, which settle undoes.
func TestSettleAfterAFailedMigration(t *testing.T) {
	d := migrated(t)
	ctx := context.Background()
	if _, err := d.Write.ExecContext(ctx, `PRAGMA foreign_keys = OFF; PRAGMA legacy_alter_table = ON;
		BEGIN IMMEDIATE; INSERT INTO users (name) VALUES ('half done')`); err != nil {
		t.Fatal(err)
	}
	if err := settle(ctx, d.Write); err != nil {
		t.Fatal(err)
	}
	var fk, legacy, users int
	if err := d.Write.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign keys %d, err %v", fk, err)
	}
	if err := d.Write.QueryRowContext(ctx, "PRAGMA legacy_alter_table").Scan(&legacy); err != nil || legacy != 0 {
		t.Errorf("legacy_alter_table %d, err %v", legacy, err)
	}
	if err := d.Write.QueryRowContext(ctx, "SELECT count(*) FROM users").Scan(&users); err != nil || users != 0 {
		t.Errorf("the half-done write stayed: %d users, err %v", users, err)
	}
	if err := settle(ctx, d.Write); err != nil {
		t.Errorf("with nothing to undo: %v", err)
	}
}

// TestMigration39 moves the sessions and goose's own record to the timestamp form: a running
// session keeps running, an ended one stays ended, and the way down gives the Julian days back.
func TestMigration39(t *testing.T) {
	d := empty(t)
	ctx := context.Background()
	p, err := provider(d.Write)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 38); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.ExecContext(ctx, `INSERT INTO sessions (token, data, expiry) VALUES
		('running', x'00', julianday('now', '+1 day')), ('ended', x'00', julianday('now', '-1 day'))`); err != nil {
		t.Fatal(err)
	}
	count := func(q string) int64 {
		t.Helper()
		var n int64
		if err := d.Write.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	versions := count("SELECT count(*) FROM goose_db_version")

	if _, err := p.UpTo(ctx, 39); err != nil {
		t.Fatal(err)
	}
	now := "strftime('%Y-%m-%dT%H:%M:%SZ', 'now')"
	for q, want := range map[string]int64{
		"SELECT count(*) FROM sessions WHERE expiry > " + now:                                          1,
		"SELECT count(*) FROM sessions WHERE token = 'running' AND expiry > " + now:                    1,
		"SELECT count(*) FROM sessions WHERE typeof(expiry) = 'text'":                                  2,
		"SELECT count(*) FROM goose_db_version":                                                        versions + 1,
		"SELECT count(*) FROM goose_db_version WHERE tstamp IS strftime('%Y-%m-%dT%H:%M:%SZ', tstamp)": versions + 1,
		"PRAGMA user_version": 39,
	} {
		if got := count(q); got != want {
			t.Errorf("after: %s = %d, want %d", q, got, want)
		}
	}
	if _, err := d.Write.ExecContext(ctx, `INSERT INTO sessions (token, data, expiry) VALUES ('bad', x'00', julianday('now'))`); err == nil {
		t.Error("a Julian day was taken as an expiry")
	}

	if _, err := p.DownTo(ctx, 38); err != nil {
		t.Fatal(err)
	}
	if got := count("SELECT count(*) FROM sessions WHERE typeof(expiry) = 'real' AND expiry > julianday('now')"); got != 1 {
		t.Errorf("down: %d running sessions as Julian days", got)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// TestMigration40 gives six tables when each row last changed, starting from when it was made,
// and the loans' view the column; the way down takes it off again.
func TestMigration40(t *testing.T) {
	d := empty(t)
	ctx := context.Background()
	p, err := provider(d.Write)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 39); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`INSERT INTO works (title) VALUES ('Dune')`,
		`INSERT INTO editions (work_id, title) VALUES (1, 'Dune')`,
		`INSERT INTO copies (edition_id) VALUES (1)`,
		`INSERT INTO borrowers (name, created_at) VALUES ('Sam', '2026-10-01T09:00:00Z')`,
		`INSERT INTO loans (copy_id, borrower_id, lent_on, created_at) VALUES (1, 1, '2026-10-01', '2026-10-01T09:00:00Z')`,
		`INSERT INTO areas (name, position) VALUES ('Fiction', 1)`,
	} {
		if _, err := d.Write.ExecContext(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if _, err := p.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	var loan, borrower, area string
	if err := d.Read.QueryRowContext(ctx, "SELECT updated_at FROM loan_rows WHERE id = 1").Scan(&loan); err != nil || loan != "2026-10-01T09:00:00Z" {
		t.Errorf("the loan's stamp through its view: %q, err %v", loan, err)
	}
	if err := d.Read.QueryRowContext(ctx, "SELECT updated_at FROM borrowers").Scan(&borrower); err != nil || borrower != "2026-10-01T09:00:00Z" {
		t.Errorf("the borrower's stamp: %q, err %v", borrower, err)
	}
	if err := d.Read.QueryRowContext(ctx, "SELECT updated_at FROM areas").Scan(&area); err != nil || area == "" {
		t.Errorf("the area's stamp: %q, err %v", area, err)
	}
	if _, err := p.DownTo(ctx, 39); err != nil {
		t.Fatal(err)
	}
	var gone string
	if err := d.Read.QueryRowContext(ctx, "SELECT updated_at FROM borrowers").Scan(&gone); err == nil {
		t.Error("down: borrowers still have updated_at")
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

func TestMigration41(t *testing.T) {
	d := empty(t)
	ctx := context.Background()
	p, err := provider(d.Write)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.UpTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`INSERT INTO works (title, category) VALUES ('Dune', 'fiction'), ('Ethics', '')`,
		`INSERT INTO editions (work_id, title) VALUES (1, 'Dune'), (2, 'Ethics'), (2, 'Ethics, 6th ed')`,
		`INSERT INTO copies (edition_id) VALUES (2)`,
		`INSERT INTO areas (name, position) VALUES ('Fiction', 1)`,
		`INSERT INTO area_categories (area_id, category) VALUES (1, 'fiction')`,
		`INSERT INTO shares (name, token_hash) VALUES ('Mom', 't1')`,
		`INSERT INTO share_categories (share_id, category) VALUES (1, 'nonfiction')`,
		`INSERT INTO courses (school, title) VALUES ('IU', 'Ethics')`,
	} {
		if _, err := d.Write.ExecContext(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if _, err := d.Write.ExecContext(ctx, `UPDATE works SET category = 'textbook' WHERE id = 2`); err == nil {
		t.Fatal("before: a textbook was taken")
	}
	if _, err := p.UpTo(ctx, 41); err != nil {
		t.Fatal(err)
	}
	count := func(q string) int64 {
		t.Helper()
		var n int64
		if err := d.Read.QueryRowContext(ctx, q).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	for q, want := range map[string]int64{
		"SELECT count(*) FROM works WHERE category = 'fiction'":                        1,
		"SELECT count(*) FROM area_categories WHERE category = 'fiction'":              1,
		"SELECT count(*) FROM share_categories WHERE category = 'nonfiction'":          1,
		"SELECT count(*) FROM pragma_foreign_key_check":                                0,
		"SELECT count(*) FROM sqlite_master WHERE name = 'works_series_idx'":           1,
		"SELECT count(*) FROM sqlite_master WHERE name = 'works_changed'":              1,
		"SELECT count(*) FROM sqlite_master WHERE name LIKE 'course_copies_changed_%'": 2,
		"PRAGMA user_version": 41,
	} {
		if got := count(q); got != want {
			t.Errorf("after: %s = %d, want %d", q, got, want)
		}
	}
	for _, s := range []string{
		`UPDATE works SET category = 'textbook' WHERE id = 1`,
		`INSERT INTO area_categories (area_id, category) VALUES (1, 'textbook')`,
		`INSERT INTO share_categories (share_id, category) VALUES (1, 'textbook')`,
	} {
		if _, err := d.Write.ExecContext(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	// A copy put on a course tells the feed of every edition of its work.
	before := count("SELECT count(*) FROM changes")
	if _, err := d.Write.ExecContext(ctx, `INSERT INTO course_copies (course_id, copy_id) VALUES (1, 1)`); err != nil {
		t.Fatal(err)
	}
	if got := count("SELECT count(*) FROM changes WHERE type = 'book' AND ref IN (2, 3)"); got < 2 || count("SELECT count(*) FROM changes") != before+2 {
		t.Errorf("on a course: %d changes for its editions", got)
	}
	if _, err := d.Write.ExecContext(ctx, `DELETE FROM course_copies`); err != nil {
		t.Fatal(err)
	}
	if got := count("SELECT count(*) FROM changes"); got != before+4 {
		t.Errorf("off a course: %d changes, want %d", got, before+4)
	}

	if _, err := p.DownTo(ctx, 40); err != nil {
		t.Fatal(err)
	}
	for q, want := range map[string]int64{
		"SELECT count(*) FROM works WHERE category = ''":                               2, // Dune's textbook, and Ethics' worked out
		"SELECT count(*) FROM area_categories":                                         1,
		"SELECT count(*) FROM share_categories":                                        1,
		"SELECT count(*) FROM sqlite_master WHERE name LIKE 'course_copies_changed_%'": 0,
	} {
		if got := count(q); got != want {
			t.Errorf("down: %s = %d, want %d", q, got, want)
		}
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
}
