// Package extract reads the words in a document's file with the host's own programs:
// pdftotext for a PDF that carries its text, and Tesseract for a photograph or a PDF of
// scanned pages that does not. It runs in the text service, apart from the web server, at a
// low priority set by that service's unit.
package extract

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrNoTools is a host without pdftotext, pdftoppm, or tesseract.
var ErrNoTools = errors.New("the text tools (poppler-utils and tesseract-ocr) are not installed")

// Tools reads text with the programs on PATH.
type Tools struct {
	Lang     string        // Tesseract's language, eng unless set
	MaxPages int           // the most pages of a scanned PDF to read, 40 unless set
	Timeout  time.Duration // the longest one file may take, ten minutes unless set
}

// Text returns the words in the file at path, of the content type given.
func (t Tools) Text(ctx context.Context, path, contentType string) (string, error) {
	if t.Timeout == 0 {
		t.Timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	if contentType != "application/pdf" {
		return t.ocr(ctx, path)
	}
	text, err := run(ctx, "pdftotext", "-enc", "UTF-8", path, "-")
	if err != nil {
		return "", err
	}
	// A PDF of scanned pages has a few stray words at most; one with its own text has many.
	if len(strings.Fields(text)) >= 25 {
		return tidy(text), nil
	}
	return t.scanned(ctx, path)
}

// scanned renders each page as an image and reads it.
func (t Tools) scanned(ctx context.Context, path string) (string, error) {
	if t.MaxPages == 0 {
		t.MaxPages = 40
	}
	dir, err := os.MkdirTemp("", "hl-pages-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if _, err := run(ctx, "pdftoppm", "-r", "300", "-gray", "-png", "-l", strconv.Itoa(t.MaxPages), path, filepath.Join(dir, "page")); err != nil {
		return "", err
	}
	pages, err := filepath.Glob(filepath.Join(dir, "page*.png"))
	if err != nil {
		return "", err
	}
	slices.Sort(pages) // pdftoppm pads the numbers, so the names sort in page order
	var all []string
	for _, p := range pages {
		text, err := t.ocr(ctx, p)
		if err != nil {
			return "", err
		}
		all = append(all, text)
	}
	return strings.Join(all, "\n\n"), nil
}

func (t Tools) ocr(ctx context.Context, path string) (string, error) {
	lang := t.Lang
	if lang == "" {
		lang = "eng"
	}
	text, err := run(ctx, "tesseract", path, "stdout", "-l", lang)
	if err != nil {
		return "", err
	}
	return tidy(text), nil
}

func run(ctx context.Context, name string, args ...string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", ErrNoTools
	}
	// The tools run at idle priority, CPU and disk, where the system has chrt and ionice; the
	// service itself does not, since it holds the database's write lock while it saves.
	if _, err := exec.LookPath("chrt"); err == nil {
		if _, err := exec.LookPath("ionice"); err == nil {
			args = append([]string{"--idle", "0", "ionice", "-c3", name}, args...)
			name = "chrt"
		}
	}
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // this package's own programs, by name
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return "", fmt.Errorf("%s: %w: %s", name, err, msg)
	}
	return out.String(), nil
}

// tidy drops form feeds and runs of blank lines, which OCR and pdftotext leave plenty of.
func tidy(s string) string {
	s = strings.ReplaceAll(s, "\f", "\n")
	var lines []string
	blank := 0
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimRight(l, " \t\r")
		if l == "" {
			if blank++; blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		lines = append(lines, l)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
