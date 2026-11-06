// Package backup keeps consistent snapshots of the library beside it on the host, one a day,
// pruned by age: the database, and the covers and document files as they stood that day.
package backup

import (
	"context"
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
// directory of the files beside it.
const (
	prefix      = "homelibrary-"
	suffix      = ".sqlite"
	filesPrefix = "files-"
	dayForm     = "2006-01-02"
)

// Run takes today's snapshot of the database through SQLite's own VACUUM INTO, which reads the
// live database consistently while the service keeps running, then of the files, and drops
// snapshots older than Keep days. It returns the database snapshot's path.
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
	dest := filepath.Join(o.Dir, prefix+day+suffix)
	// A second run on the same day replaces the morning's file: VACUUM INTO refuses to
	// overwrite, so the old one goes first.
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("replace snapshot: %w", err)
	}
	if _, err := write.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	files, err := snapshotFiles(filepath.Join(o.Dir, filesPrefix+day), o.Files)
	if err != nil {
		return "", err
	}
	pruned, err := prune(o.Dir, o.Now.AddDate(0, 0, -o.Keep))
	if err != nil {
		return "", err
	}
	o.Logger.Info("backup", "snapshot", dest, "bytes", info.Size(), "files", files, "pruned", pruned)
	return dest, nil
}

// snapshotFiles links every file under each directory into dest, under the directory's name.
// Covers and document files are written once under new names and never changed, so a hard
// link holds a file as it was for as long as the snapshot stays, and takes no room until the
// library lets the file go. A file that cannot be linked, on another disk, is copied. Names
// starting with a dot are uploads still being written.
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

// prune removes dated snapshots, of the database and of the files, from before the cutoff,
// and leaves everything else in the directory, such as the deploy's pre-release copies, alone.
func prune(dir string, before time.Time) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("backup dir: %w", err)
	}
	n := 0
	for _, e := range entries {
		var stamp string
		if name, ok := strings.CutPrefix(e.Name(), prefix); ok && !e.IsDir() {
			stamp, ok = strings.CutSuffix(name, suffix)
			if !ok {
				continue
			}
		} else if name, ok := strings.CutPrefix(e.Name(), filesPrefix); ok && e.IsDir() {
			stamp = name
		} else {
			continue
		}
		day, err := time.Parse(dayForm, stamp)
		if err != nil || !day.Before(before) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return n, fmt.Errorf("prune %s: %w", e.Name(), err)
		}
		n++
	}
	return n, nil
}
