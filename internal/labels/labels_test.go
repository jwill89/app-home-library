package labels

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestCode128Table(t *testing.T) {
	seen := map[string]bool{}
	for i, p := range code128 {
		want := 11
		if i == stop {
			want = 13
		}
		sum := 0
		for _, c := range p {
			sum += int(c - '0')
		}
		if sum != want {
			t.Errorf("symbol %d (%s) is %d modules, want %d", i, p, sum, want)
		}
		if seen[p] {
			t.Errorf("symbol %d (%s) repeats another", i, p)
		}
		seen[p] = true
	}
	if len(code128) != 107 {
		t.Fatalf("%d symbols, want 107", len(code128))
	}
}

func TestCode128(t *testing.T) {
	widths, err := Code128("HL00001")
	if err != nil {
		t.Fatal(err)
	}
	// Start B, seven characters, the check symbol, and the stop.
	if got := modules(widths); got != 11*9+13 {
		t.Fatalf("%d modules", got)
	}
	// H, L, four zeros, and a one weigh in at 104 + 40·1 + 44·2 + 16·(3+4+5+6) + 17·7 = 639, and
	// 639 mod 103 is 21.
	var flat strings.Builder
	for _, w := range widths {
		flat.WriteString(strconv.Itoa(w))
	}
	s := flat.String()
	if !strings.HasPrefix(s, code128[startB]) || !strings.HasSuffix(s, code128[21]+code128[stop]) {
		t.Fatalf("encoding %s", s)
	}
	if _, err := Code128("é"); err == nil {
		t.Fatal("a character outside ASCII was encoded")
	}
}

// xrefOK checks that every offset in the cross-reference table points at its object.
func xrefOK(t *testing.T, pdf []byte) {
	t.Helper()
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatal("not a PDF file")
	}
	start := regexp.MustCompile(`startxref\n(\d+)`).FindSubmatch(pdf)
	at, _ := strconv.Atoi(string(start[1]))
	if !bytes.HasPrefix(pdf[at:], []byte("xref\n")) {
		t.Fatal("startxref does not point at the table")
	}
	for i, m := range regexp.MustCompile(`(\d{10}) 00000 n `).FindAllSubmatch(pdf, -1) {
		off, _ := strconv.Atoi(string(m[1]))
		if want := strconv.Itoa(i+1) + " 0 obj"; !bytes.HasPrefix(pdf[off:], []byte(want)) {
			t.Fatalf("object %d is not at %d", i+1, off)
		}
	}
}

func TestRender(t *testing.T) {
	items := make([]Label, 81)
	for i := range items {
		items[i] = Label{Barcode: "HL0000" + strconv.Itoa(i%10), Title: "Über (Fellowship) — 2", CallNumber: []string{"FIC", "TOL", "v.1"}}
	}
	pdf, err := Render(Barcode, items, Offset{X: 1, Y: -2})
	if err != nil {
		t.Fatal(err)
	}
	xrefOK(t, pdf)
	if n := bytes.Count(pdf, []byte("/Type /Page ")); n != 2 {
		t.Fatalf("81 barcode labels on %d sheets, want 2", n)
	}
	// Parentheses are escaped; the umlaut, the dash, and the ellipsis that ends a caption cut
	// to fit are WinAnsi bytes.
	if !bytes.Contains(pdf, []byte(`(HL00000  \334ber \(Fellowship\) \227\205) Tj`)) {
		t.Fatalf("caption not encoded: %s", pdf)
	}
	spines, err := Render(Spine, items[:60], Offset{})
	if err != nil {
		t.Fatal(err)
	}
	xrefOK(t, spines)
	if n := bytes.Count(spines, []byte("(TOL) Tj")); n != 60 {
		t.Fatalf("%d spine labels", n)
	}
	xrefOK(t, Alignment(Spine, Offset{X: -1, Y: 3}))
}

func TestFit(t *testing.T) {
	if got := fit("Blackwing", 10, 1000); got != "Blackwing" {
		t.Fatal(got)
	}
	// Five characters of 6 points fit in 30: four and the ellipsis.
	if got := fit("Blackwing", 10, 30); got != "Blac…" {
		t.Fatalf("cut to %q", got)
	}
}
