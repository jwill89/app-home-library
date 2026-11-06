// Package files keeps documents' files: the PDFs and page photographs the owner uploads. They
// live in their own directory under random names, apart from the covers, which are served to
// anyone signed in or visiting; a document's file is served only through the API, which asks
// whether the reader may have it.
package files

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Types are the files a document can have, by content type, with the extension each is kept
// under.
var Types = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/webp":      "webp",
}

var (
	// ErrUnsupported is a file that is not a PDF or a JPEG, PNG, or WebP image.
	ErrUnsupported = errors.New("not a PDF or an image")
	// ErrTooLarge is a file over the limit.
	ErrTooLarge = errors.New("file too large")

	nameRe = regexp.MustCompile(`^[a-z0-9]{26}\.(pdf|jpg|png|webp)$`)
)

// Store keeps files in Dir.
type Store struct {
	Dir string
}

// NewStore creates the directory if needed.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("documents dir: %w", err)
	}
	return &Store{Dir: dir}, nil
}

// Save writes what r holds, up to limit bytes, under a new random name, and says what kind
// of file it is by its first bytes, never by what the browser claimed.
func (s *Store) Save(r io.Reader, limit int64) (name, contentType string, size int64, err error) {
	head := make([]byte, 512)
	n, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", "", 0, fmt.Errorf("read upload: %w", err)
	}
	head = head[:n]
	contentType = Sniff(head)
	ext, ok := Types[contentType]
	if !ok {
		return "", "", 0, ErrUnsupported
	}
	name = strings.ToLower(rand.Text()) + "." + ext
	tmp, err := os.CreateTemp(s.Dir, ".upload-*")
	if err != nil {
		return "", "", 0, err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmp.Name())
		}
	}()
	size, err = io.Copy(tmp, io.LimitReader(io.MultiReader(bytes.NewReader(head), r), limit+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", "", 0, fmt.Errorf("write upload: %w", err)
	}
	if size > limit {
		return "", "", 0, ErrTooLarge
	}
	if err = os.Rename(tmp.Name(), filepath.Join(s.Dir, name)); err != nil {
		return "", "", 0, err
	}
	return name, contentType, size, nil
}

// Sniff names a file's type by its first bytes: PDF, JPEG, PNG, or WebP, or empty for
// anything else.
func Sniff(head []byte) string {
	switch t := http.DetectContentType(head); {
	case strings.HasPrefix(t, "application/pdf"):
		return "application/pdf"
	case t == "image/jpeg" || t == "image/png" || t == "image/webp":
		return t
	}
	return ""
}

// Path is where a stored file is, for one of this store's names.
func (s *Store) Path(name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", fmt.Errorf("not a stored file name: %q", name)
	}
	return filepath.Join(s.Dir, name), nil
}

// Remove deletes a stored file; one already gone is not an error.
func (s *Store) Remove(name string) error {
	p, err := s.Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
