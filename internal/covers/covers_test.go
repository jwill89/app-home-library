package covers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func pngOf(t *testing.T, w, h int) *bytes.Reader {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b.Bytes())
}

func newStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestProcess(t *testing.T) {
	store := newStore(t)
	res, err := store.Process(context.Background(), pngOf(t, 600, 900))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 600 || res.Height != 900 {
		t.Errorf("master %dx%d", res.Width, res.Height)
	}
	for _, f := range []string{"master.jpg", "w200.jpg", "w400.jpg"} {
		if len(res.Files[f]) == 0 {
			t.Errorf("missing %s", f)
		}
	}
	if _, ok := res.Files["w800.jpg"]; ok {
		t.Error("a rendition wider than the master")
	}

	name, err := store.Save(res)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Sizes(name); len(got) != 2 || got[0] != 200 {
		t.Errorf("sizes %v", got)
	}
}

func TestProcessRefusesPlaceholders(t *testing.T) {
	store := newStore(t)
	if _, err := store.Process(context.Background(), pngOf(t, 1, 1)); !errors.Is(err, ErrTooSmall) {
		t.Errorf("a 1x1 placeholder: %v", err)
	}
	if _, err := store.Process(context.Background(), bytes.NewReader([]byte("not an image"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("text: %v", err)
	}
}

// A cover's main colour is the one most of it is, not the mix of all of it: a navy cover with
// a gold title is navy.
func TestMainColor(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 150))
	for y := range 150 {
		for x := range 100 {
			c := color.RGBA{20, 30, 90, 255}
			if y > 40 && y < 70 {
				c = color.RGBA{220, 180, 40, 255}
			}
			img.Set(x, y, c)
		}
	}
	if got := MainColor(img); got != "#141e5a" {
		t.Errorf("main colour %s, want the navy #141e5a", got)
	}
	if got := MainColor(image.NewRGBA(image.Rect(0, 0, 0, 0))); got != "" {
		t.Errorf("an empty picture's colour %q", got)
	}
}

// A stored cover's colour is read from its smallest picture once, and forgotten with it.
func TestColor(t *testing.T) {
	store := newStore(t)
	if store.Color("") != "" || store.Color("../etc") != "" || store.Color("missing") != "" {
		t.Fatal("a colour for no cover")
	}
	res, err := store.Process(context.Background(), pngOf(t, 600, 900))
	if err != nil {
		t.Fatal(err)
	}
	name, err := store.Save(res)
	if err != nil {
		t.Fatal(err)
	}
	c := store.Color(name)
	if len(c) != 7 || c[0] != '#' || string(res.Files[colorFile]) != c {
		t.Fatalf("colour %q, kept as %q", c, res.Files[colorFile])
	}

	// A cover stored before colours were kept has its colour read once, and kept.
	older := newStore(t)
	older.Dir = store.Dir
	kept := filepath.Join(store.Dir, name, colorFile)
	if err := os.Remove(kept); err != nil {
		t.Fatal(err)
	}
	read := older.Color(name)
	if b, err := os.ReadFile(kept); err != nil || string(b) != read || len(read) != 7 {
		t.Fatalf("read %q, kept %q, %v", read, b, err)
	}
	if again := older.Color(name); again != read {
		t.Errorf("asked again: %q, then %q", read, again)
	}
	if err := older.Remove(name); err != nil {
		t.Fatal(err)
	}
	if got := older.Color(name); got != "" {
		t.Errorf("a removed cover's colour %q", got)
	}
}
