package covers

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Store keeps each cover's files in a directory of its own under Dir.
type Store struct {
	Dir string
	// The widths each cover has, by name: a name's files never change, so they are looked
	// for once, not three times for every book each time the library is listed.
	sizes sync.Map
}

// NewStore creates the directory if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("media dir: %w", err)
	}
	return &Store{Dir: dir}, nil
}

// NewName is a random name, so a cover's address says nothing and is never reused: a new
// cover is a new name, which is what lets every file be cached for good.
func NewName() string {
	return strings.ToLower(rand.Text())
}

var (
	itemName = regexp.MustCompile(`^[a-z0-9]{1,32}$`)
	fileName = regexp.MustCompile(`^(master|w[0-9]{2,4})\.jpg$`)
)

func (s *Store) path(name string) (string, error) {
	if !itemName.MatchString(name) {
		return "", fmt.Errorf("cover name %q", name)
	}
	return filepath.Join(s.Dir, name), nil
}

// Save writes a cover's files under a new name and returns it.
func (s *Store) Save(res *Result) (string, error) {
	name := NewName()
	dir, err := s.path(name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	for file, data := range res.Files {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o600); err != nil {
			return "", errors.Join(err, os.RemoveAll(dir))
		}
	}
	return name, nil
}

// Remove deletes a cover's files. An empty name is no cover, and nothing to do.
func (s *Store) Remove(name string) error {
	if name == "" {
		return nil
	}
	dir, err := s.path(name)
	if err != nil {
		return err
	}
	s.sizes.Delete(name)
	return os.RemoveAll(dir)
}

// Sizes lists the widths a stored cover has below its master, smallest first.
func (s *Store) Sizes(name string) []int {
	if known, ok := s.sizes.Load(name); ok {
		return known.([]int)
	}
	dir, err := s.path(name)
	if err != nil {
		return nil
	}
	var out []int
	for _, w := range Widths {
		if _, err := os.Stat(filepath.Join(dir, FileName(w))); err == nil {
			out = append(out, w)
		}
	}
	out = slices.Clip(out)
	// A cover not there (yet, or put back from a backup by hand) is looked for again next time.
	if _, err := os.Stat(dir); err == nil {
		s.sizes.Store(name, out)
	}
	return out
}

// Handler serves "GET /media/{item}/{file}". The files never change under a name, so they
// are cached for a year; private, since the library is.
func (s *Store) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dir, err := s.path(r.PathValue("item"))
		file := r.PathValue("file")
		if err != nil || !fileName.MatchString(file) {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(dir, file)) //nolint:gosec // dir and file are both checked above
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer func() { _ = f.Close() }() // opened to read: closing it has nothing to report
		info, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
		http.ServeContent(w, r, file, info.ModTime(), f)
	})
}
