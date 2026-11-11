// Package backup keeps consistent snapshots of the library beside it on the host, one a day,
// pruned by age: the database, compressed with gzip, and the covers and document files as
// they stood that day.
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite" // the driver the integrity check opens the copy with
)

// Options is where the snapshots go, what they hold, and how long they stay.
type Options struct {
	Dir    string            // where the daily snapshots land
	Keep   int               // days of snapshots kept
	Files  map[string]string // directories of files kept with each snapshot, by their name in it
	Now    time.Time         // zero means now
	Logger *slog.Logger
}

// Snapshot names carry their day, which is what the pruning reads: a database file, and a
// directory of the files beside it. Database snapshots made before they were compressed end
// in the plain suffix until a run compresses them.
const (
	prefix      = "homelibrary-"
	plain       = ".sqlite"
	compressed  = plain + ".gz"
	filesPrefix = "files-"
	dayForm     = "2006-01-02"
)

// Run takes today's snapshot of the database through SQLite's own VACUUM INTO, which reads the
// live database consistently while the service keeps running, and keeps it compressed; then of
// the files. It drops snapshots older than Keep days and compresses any database snapshot left
// from before compression. It returns the database snapshot's path.
func Run(ctx context.Context, write *sql.DB, o Options) (string, error) {
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if err := os.MkdirAll(o.Dir, 0o750); err != nil {
		return "", fmt.Errorf("backup dir: %w", err)
	}
	day := o.Now.Format(dayForm)
	dest := filepath.Join(o.Dir, prefix+day+compressed)
	size, raw, err := snapshot(ctx, write, dest)
	if err != nil {
		return "", err
	}
	// A second run on the same day replaces the morning's snapshot, in either form.
	if err := remove(filepath.Join(o.Dir, prefix+day+plain)); err != nil {
		return "", fmt.Errorf("replace snapshot: %w", err)
	}
	files, err := snapshotFiles(filepath.Join(o.Dir, filesPrefix+day), o.Files)
	if err != nil {
		return "", err
	}
	pruned, err := prune(o.Dir, o.Now.AddDate(0, 0, -o.Keep))
	if err != nil {
		return "", err
	}
	converted, err := compressOld(o.Dir)
	if err != nil {
		return "", err
	}
	o.Logger.Info("backup", "snapshot", dest, "bytes", size, "uncompressed", raw, "files", files,
		"pruned", pruned, "converted", converted)
	return dest, nil
}

// snapshot writes the live database to dest, compressed. The copy is made and checked in a
// temporary file beside dest, so dest only ever holds a whole, checked snapshot, and the
// temporary file goes whether the snapshot is made or not.
func snapshot(ctx context.Context, write *sql.DB, dest string) (size, raw int64, err error) {
	f, err := os.CreateTemp(filepath.Dir(dest), strings.TrimSuffix(filepath.Base(dest), ".gz")+".*.partial")
	if err != nil {
		return 0, 0, fmt.Errorf("snapshot: %w", err)
	}
	tmp := f.Name()
	defer func() { err = errors.Join(err, remove(tmp)) }()
	if err := f.Close(); err != nil {
		return 0, 0, fmt.Errorf("snapshot: %w", err)
	}
	// VACUUM INTO writes only to a file that is missing or empty, as this one is.
	if _, err := write.ExecContext(ctx, "VACUUM INTO ?", tmp); err != nil {
		return 0, 0, fmt.Errorf("snapshot: %w", err)
	}
	if err := check(ctx, tmp); err != nil {
		return 0, 0, err
	}
	return compress(tmp, dest)
}

// check runs SQLite's integrity check over a copy, opened read-only so that nothing is
// written beside it.
func check(ctx context.Context, path string) (err error) {
	copied, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("check snapshot: %w", err)
	}
	defer func() { err = errors.Join(err, copied.Close()) }()
	var verdict string
	if err := copied.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&verdict); err != nil {
		return fmt.Errorf("check snapshot: %w", err)
	}
	if verdict != "ok" {
		return fmt.Errorf("snapshot failed its integrity check, so the earlier snapshots stay as they are: %s", verdict)
	}
	return nil
}

// compress gzips src into a temporary file beside dest, reads that back to be sure it
// gives src again, and renames it into place. It returns the sizes of dest and src. src is
// left as it was, and the temporary file goes whether dest is written or not.
func compress(src, dest string) (size, raw int64, err error) {
	in, err := os.Open(src) //nolint:gosec // a snapshot in the backup folder, named by Run
	if err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	defer func() { _ = in.Close() }() // opened to read: closing it has nothing to report
	out, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*.partial")
	if err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	tmp := out.Name()
	defer func() { err = errors.Join(err, remove(tmp)) }()

	sum := sha256.New()
	zw := gzip.NewWriter(out)
	raw, err = io.Copy(zw, io.TeeReader(in, sum))
	if err == nil {
		err = zw.Close()
	}
	if err == nil {
		// On disk before it takes the name, so a crash cannot leave a short file under it.
		err = out.Sync()
	}
	if err := errors.Join(err, out.Close()); err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	if err := readBack(tmp, raw, sum.Sum(nil)); err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	info, err := os.Stat(tmp)
	if err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return 0, 0, fmt.Errorf("compress %s: %w", filepath.Base(src), err)
	}
	return info.Size(), raw, nil
}

// readBack decompresses a file, which checks gzip's own checksum and length, and compares
// what comes out with the original's length and digest.
func readBack(path string, size int64, want []byte) error {
	f, err := os.Open(path) //nolint:gosec // the temporary file compress just wrote
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }() // opened to read: closing it has nothing to report
	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	sum := sha256.New()
	// One byte past the original's length is as far as it needs reading to show where it ends.
	n, err := io.Copy(sum, io.LimitReader(zr, size+1))
	if err != nil {
		return fmt.Errorf("read back: %w", err)
	}
	if n != size || !bytes.Equal(sum.Sum(nil), want) {
		return errors.New("read back: the compressed copy does not give the snapshot again")
	}
	return zr.Close()
}

// compressOld compresses the dated database snapshots made before snapshots were compressed,
// each keeping its day and its time, and removes the uncompressed file once its copy is in place.
func compressOld(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("backup dir: %w", err)
	}
	n := 0
	for _, e := range entries {
		if _, form, ok := dated(e); !ok || form != plain {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return n, fmt.Errorf("compress %s: %w", e.Name(), err)
		}
		src := filepath.Join(dir, e.Name())
		dest := src + ".gz"
		if _, _, err := compress(src, dest); err != nil {
			return n, err
		}
		if err := os.Chtimes(dest, info.ModTime(), info.ModTime()); err != nil {
			return n, fmt.Errorf("compress %s: %w", e.Name(), err)
		}
		if err := os.Remove(src); err != nil {
			return n, fmt.Errorf("compress %s: %w", e.Name(), err)
		}
		n++
	}
	return n, nil
}

// snapshotFiles links every file under each directory into dest, under the directory's name.
// Covers and document files are written once under new names and never changed, so a hard
// link holds a file as it was for as long as the snapshot stays, and takes no room until the
// library lets the file go; that is also why they are not compressed. A file that cannot be
// linked, on another disk, is copied. Names starting with a dot are uploads still being written.
func snapshotFiles(dest string, dirs map[string]string) (int, error) {
	// A second run on the same day starts afresh.
	if err := os.RemoveAll(dest); err != nil {
		return 0, fmt.Errorf("replace files snapshot: %w", err)
	}
	n := 0
	names := make([]string, 0, len(dirs))
	for name := range dirs {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		src := dirs[name]
		err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == src && errors.Is(err, fs.ErrNotExist) {
					return nil // nothing kept there yet
				}
				return err
			}
			if strings.HasPrefix(d.Name(), ".") && p != src {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			to := filepath.Join(dest, name, rel)
			if err := os.MkdirAll(filepath.Dir(to), 0o750); err != nil {
				return err
			}
			if err := os.Link(p, to); err != nil { //nolint:gosec // the library's own directories, written by the service alone; only regular files reach here
				if err := copyFile(p, to); err != nil {
					return err
				}
			}
			n++
			return nil
		})
		if err != nil {
			return n, fmt.Errorf("files snapshot %s: %w", name, err)
		}
	}
	return n, nil
}

func copyFile(from, to string) (err error) {
	in, err := os.Open(from) //nolint:gosec // a file under the library's own directories
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()                                     // opened to read
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // a path inside the backup directory
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	_, err = io.Copy(out, in)
	return err
}

// prune removes dated snapshots from before the cutoff: the database's in either form, and
// the files' directories. Everything else in the directory, such as the deploy's pre-release
// copies, stays.
func prune(dir string, before time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("backup dir: %w", err)
	}
	n := 0
	for _, e := range entries {
		day, _, ok := dated(e)
		if !ok || !day.Before(before) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return n, fmt.Errorf("prune %s: %w", e.Name(), err)
		}
		n++
	}
	return n, nil
}

// dated reads the day from a daily snapshot's exact name and says which form it is in: a
// database file, compressed or plain, or a directory of files. No other name in the folder,
// the deploy's homelibrary-pre-* snapshots included, has one.
func dated(e fs.DirEntry) (time.Time, string, bool) {
	if e.IsDir() {
		d, ok := strings.CutPrefix(e.Name(), filesPrefix)
		if !ok {
			return time.Time{}, "", false
		}
		day, err := time.Parse(dayForm, d)
		return day, filesPrefix, err == nil
	}
	rest, ok := strings.CutPrefix(e.Name(), prefix)
	if !ok {
		return time.Time{}, "", false
	}
	for _, form := range []string{compressed, plain} {
		if d, cut := strings.CutSuffix(rest, form); cut {
			day, err := time.Parse(dayForm, d)
			return day, form, err == nil
		}
	}
	return time.Time{}, "", false
}

// remove deletes a file that may already be gone.
func remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
