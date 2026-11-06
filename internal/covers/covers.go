// Package covers turns a cover image, fetched from a source or photographed by the owner,
// into JPEG renditions stored on disk under a random name. JPEG through the standard
// library, not WebP: the pure-Go WebP encoder crashed on a cover 333 pixels wide
// (docs/decisions.md, 2026-11-04).
package covers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // decoders, which register themselves with image
	"image/jpeg"
	_ "image/png" //
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // a WebP upload, decoded like the rest
)

const (
	// MaxBytes caps an upload and a download.
	MaxBytes  = 12 << 20
	maxPixels = 40_000_000
	// A cover is shown at most a few hundred pixels wide; the master is for a closer look.
	masterMax = 1400
	quality   = 82
	// Smaller than this is a placeholder or a thumbnail not worth keeping.
	minHeight = 150
)

// Widths are the renditions kept below the master, for lists, cards, and the book's page.
var Widths = []int{200, 400, 800}

var (
	ErrTooLarge    = errors.New("image is too large")
	ErrTooSmall    = errors.New("image is too small to be a cover")
	ErrUnsupported = errors.New("not a supported image")
)

// Result is the encoded file set and the master's size.
type Result struct {
	Width  int
	Height int
	Files  map[string][]byte
}

// FileName is the stored name of a rendition.
func FileName(width int) string {
	return fmt.Sprintf("w%d.jpg", width)
}

// One image is decoded and encoded at a time: a decoded photo is its whole bitmap, and the
// droplet has little memory to spare.
var gate = make(chan struct{}, 1)

// Process reads an image and encodes its renditions. Decoding and encoding again is also
// what strips the metadata a phone photo carries, its location included.
func Process(ctx context.Context, r io.ReadSeeker) (*Result, error) {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, ErrTooLarge
	}
	if cfg.Height < minHeight {
		return nil, ErrTooSmall
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-gate }()
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	master := fit(src, masterMax)
	w, h := master.Bounds().Dx(), master.Bounds().Dy()
	files := map[string][]byte{}
	b, err := encode(master)
	if err != nil {
		return nil, fmt.Errorf("encode master: %w", err)
	}
	files["master.jpg"] = b
	for _, dw := range Widths {
		if dw >= w {
			break
		}
		b, err := encode(scale(master, dw, max(1, dw*h/w)))
		if err != nil {
			return nil, fmt.Errorf("encode %dpx: %w", dw, err)
		}
		files[FileName(dw)] = b
	}
	return &Result{Width: w, Height: h, Files: files}, nil
}

// Download fetches an image from a source's cover link and processes it.
func Download(ctx context.Context, client *http.Client, userAgent, url string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() // read in full below
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover %s", resp.Status)
	}
	if t := resp.Header.Get("Content-Type"); t != "" && !strings.HasPrefix(t, "image/") {
		return nil, fmt.Errorf("%w: %s", ErrUnsupported, t)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxBytes {
		return nil, ErrTooLarge
	}
	return Process(ctx, bytes.NewReader(body))
}

func encode(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := jpeg.Encode(&b, img, &jpeg.Options{Quality: quality})
	return b.Bytes(), err
}

// fit scales an image down so neither side exceeds longest.
func fit(src image.Image, longest int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	switch {
	case w <= longest && h <= longest:
		return scale(src, w, h)
	case w >= h:
		return scale(src, longest, longest*h/w)
	default:
		return scale(src, longest*w/h, longest)
	}
}

func scale(src image.Image, w, h int) *image.NRGBA {
	w, h = max(w, 1), max(h, 1)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
