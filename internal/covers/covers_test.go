package covers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
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

func TestProcess(t *testing.T) {
	res, err := Process(context.Background(), pngOf(t, 600, 900))
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

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
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
	if _, err := Process(context.Background(), pngOf(t, 1, 1)); !errors.Is(err, ErrTooSmall) {
		t.Errorf("a 1x1 placeholder: %v", err)
	}
	if _, err := Process(context.Background(), bytes.NewReader([]byte("not an image"))); !errors.Is(err, ErrUnsupported) {
		t.Errorf("text: %v", err)
	}
}
