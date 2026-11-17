// Package covers turns a cover image, fetched from a source or photographed by the owner,
// into JPEG renditions stored on disk under a random name. JPEG through the standard
// library, not WebP: the pure-Go WebP encoder crashed on a cover 333 pixels wide.
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

// Process reads an image and encodes its renditions. Decoding and encoding again is also
// what strips the metadata a phone photo carries, its location included.
func (s *Store) Process(ctx context.Context, r io.ReadSeeker) (*Result, error) {
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
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.gate }()
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
	// The spine's colour, from a picture small enough to read in a moment.
	files[colorFile] = []byte(MainColor(scale(master, 100, max(1, 100*h/w))))
	return &Result{Width: w, Height: h, Files: files}, nil
}

// Download fetches an image from a source's cover link and processes it.
func (s *Store) Download(ctx context.Context, client *http.Client, userAgent, url string) (*Result, error) {
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
	return s.Process(ctx, bytes.NewReader(body))
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

// MainColor is the colour most of a picture is, as #rrggbb: its pixels are put into bins of
// 32 shades a channel, near enough that the eye takes them for one colour, and the fullest
// bin's average is the answer. An average of the whole picture would be the muddy brown of
// every colour mixed; a cover's main colour is the one its spine most likely shares.
func MainColor(img image.Image) string {
	type bin struct{ n, r, g, b int }
	var bins [512]bin
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r, g, b = r>>8, g>>8, b>>8
			i := (r>>5)<<6 | (g>>5)<<3 | b>>5
			bins[i].n++
			bins[i].r += int(r)
			bins[i].g += int(g)
			bins[i].b += int(b)
		}
	}
	most := bins[0]
	for _, b := range bins[1:] {
		if b.n > most.n {
			most = b
		}
	}
	if most.n == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x", most.r/most.n, most.g/most.n, most.b/most.n)
}
