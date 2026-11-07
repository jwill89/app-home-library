package backup

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"homelibrary/internal/db"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	database, err := db.Open(filepath.Join(root, "live.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	media, docs, dir := filepath.Join(root, "media"), filepath.Join(root, "documents"), filepath.Join(root, "backups")
	write := func(p, s string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(media, "abc", "master.jpg"), "cover")
	write(filepath.Join(docs, "x1y2"), "article")
	write(filepath.Join(docs, ".upload-123"), "half")

	// An old snapshot of each kind, past keeping, in both of the database's forms; a recent one
	// from before compression, which a run compresses; and a deploy's copies, never touched.
	write(filepath.Join(dir, "homelibrary-2026-09-01.sqlite"), "old")
	write(filepath.Join(dir, "homelibrary-2026-09-02.sqlite.gz"), "old")
	write(filepath.Join(dir, "files-2026-09-01", "media", "z", "master.jpg"), "old")
	write(filepath.Join(dir, "homelibrary-2026-11-01.sqlite"), "recent")
	recentTime := time.Date(2026, 11, 1, 9, 31, 0, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "homelibrary-2026-11-01.sqlite"), recentTime, recentTime); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(dir, "homelibrary-pre-20260901-120000.sqlite"), "deploy")
	write(filepath.Join(dir, "homelibrary-pre-20261101-120000.sqlite.gz"), "deploy")

	o := Options{
		Dir: dir, Keep: 28, Now: time.Date(2026, 11, 5, 9, 30, 0, 0, time.UTC),
		Files:  map[string]string{"media": media, "documents": docs},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	path, err := Run(ctx, database.Write, o)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "homelibrary-2026-11-05.sqlite.gz" {
		t.Errorf("snapshot %s", path)
	}
	// The snapshot decompresses to a whole database, and nothing half-written is left beside it.
	restored := filepath.Join(root, "restored.sqlite")
	gunzip(t, path, restored)
	copied, err := db.Open(restored)
	if err != nil {
		t.Fatal(err)
	}
	var verdict string
	if err := copied.Write.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&verdict); err != nil || verdict != "ok" {
		t.Errorf("restored snapshot: %q, err %v", verdict, err)
	}
	_ = copied.Close()
	if partial, _ := filepath.Glob(filepath.Join(dir, "*.partial")); len(partial) > 0 {
		t.Errorf("left behind: %v", partial)
	}
	files := filepath.Join(dir, "files-2026-11-05")
	for rel, want := range map[string]string{"media/abc/master.jpg": "cover", "documents/x1y2": "article"} {
		if got, err := os.ReadFile(filepath.Join(files, rel)); err != nil || string(got) != want {
			t.Errorf("%s = %q, err %v", rel, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(files, "documents", ".upload-123")); !os.IsNotExist(err) {
		t.Errorf("an upload still being written was kept: %v", err)
	}

	// The library lets a file go; the snapshot keeps it as it was.
	if err := os.Remove(filepath.Join(docs, "x1y2")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(files, "documents", "x1y2")); err != nil || string(got) != "article" {
		t.Errorf("after the library let it go: %q, err %v", got, err)
	}

	for name, kept := range map[string]bool{
		"homelibrary-2026-09-01.sqlite": false, "homelibrary-2026-09-02.sqlite.gz": false,
		"files-2026-09-01":              false,
		"homelibrary-2026-11-01.sqlite": false, "homelibrary-2026-11-01.sqlite.gz": true,
		"homelibrary-pre-20260901-120000.sqlite":    true,
		"homelibrary-pre-20261101-120000.sqlite.gz": true,
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != kept {
			t.Errorf("%s kept %v, err %v", name, kept, err)
		}
	}
	// The recent plain snapshot was compressed with its time kept, and gives itself again.
	recent := filepath.Join(dir, "homelibrary-2026-11-01.sqlite.gz")
	if info, err := os.Stat(recent); err != nil || !info.ModTime().Equal(recentTime) {
		t.Errorf("compressed copy's time: %v, err %v", info, err)
	}
	gunzip(t, recent, filepath.Join(root, "recent.sqlite"))
	if got, err := os.ReadFile(filepath.Join(root, "recent.sqlite")); err != nil || string(got) != "recent" {
		t.Errorf("compressed copy gives %q, err %v", got, err)
	}

	// A second run the same day starts the files afresh, without the one let go.
	if _, err := Run(ctx, database.Write, o); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(files, "documents", "x1y2")); !os.IsNotExist(err) {
		t.Errorf("a rerun kept a file the library had let go: %v", err)
	}
}

// gunzip decompresses from into to.
func gunzip(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from) //nolint:gosec // a snapshot the test just made
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	zr, err := gzip.NewReader(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.Create(to) //nolint:gosec // a path in the test's own temp folder
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, zr); err != nil { //nolint:gosec // the test's own small snapshot
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}
