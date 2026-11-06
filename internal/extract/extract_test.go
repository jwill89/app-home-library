package extract

import (
	"context"
	"errors"
	"os/exec"
	"testing"
)

func TestTidy(t *testing.T) {
	in := "Office hours\f\n\n\n\nTuesday  \r\n\n\nRoom 4\n\n"
	if got := tidy(in); got != "Office hours\n\nTuesday\n\nRoom 4" {
		t.Fatalf("tidied to %q", got)
	}
}

// On a machine without the tools, as a Windows workstation is, the service is told so plainly
// and waits rather than failing every file.
func TestWithoutTools(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err == nil {
		t.Skip("tesseract is installed here")
	}
	if _, err := (Tools{}).Text(context.Background(), "page.png", "image/png"); !errors.Is(err, ErrNoTools) {
		t.Fatalf("got %v, want ErrNoTools", err)
	}
}
