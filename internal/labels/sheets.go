package labels

import (
	"fmt"
	"strings"
)

// Sheet is a label sheet's layout, in points from the page's top left corner.
type Sheet struct {
	Kind     string `json:"kind" enum:"barcode,spine"`
	Name     string `json:"name"`
	Across   int    `json:"across"`
	Down     int    `json:"down"`
	PerSheet int    `json:"perSheet"`
	w, h     float64
	left     float64
	top      float64
	pitchX   float64
	pitchY   float64
}

// The two sheets, from Avery's templates. Barcode labels go inside a book's front cover on
// 6467, whose adhesive peels off clean; it is 5167's template, as the label databases of
// gLabels and LibreOffice give it (5167 itself is permanent). 5195 is taken to share its
// columns, as Avery's 1¾-inch sheets do, with rows of two thirds of an inch. The alignment
// sheet shows how a printer lands on either.
var (
	Barcode = Sheet{Kind: "barcode", Name: "Avery 6467", Across: 4, Down: 20, PerSheet: 80,
		w: 1.75 * inch, h: 0.5 * inch, left: 0.28125 * inch, top: 0.5 * inch, pitchX: 2.0625 * inch, pitchY: 0.5 * inch}
	Spine = Sheet{Kind: "spine", Name: "Avery 5195", Across: 4, Down: 15, PerSheet: 60,
		w: 1.75 * inch, h: inch * 2 / 3, left: 0.28125 * inch, top: 0.5 * inch, pitchX: 2.0625 * inch, pitchY: inch * 2 / 3}
)

// SheetFor returns the sheet a kind of label is printed on.
func SheetFor(kind string) (Sheet, bool) {
	switch kind {
	case Barcode.Kind:
		return Barcode, true
	case Spine.Kind:
		return Spine, true
	}
	return Sheet{}, false
}

// Label is what one label carries.
type Label struct {
	Barcode    string   // HL00042
	Title      string   // printed small beside the barcode's number, to match label to book
	CallNumber []string // the spine label's lines
}

// Offset moves everything printed on a sheet, in millimetres: right and down are positive.
type Offset struct {
	X, Y int
}

// slot is where the label at position i (from 0, across then down) sits on its page.
func (s Sheet) slot(i int, off Offset) (x, y float64) {
	i %= s.PerSheet
	return s.left + float64(i%s.Across)*s.pitchX + float64(off.X)*mm,
		s.top + float64(i/s.Across)*s.pitchY + float64(off.Y)*mm
}

// Render draws the labels on as many sheets as they need, in order, across then down.
func Render(s Sheet, items []Label, off Offset) ([]byte, error) {
	var pages []*page
	for i, l := range items {
		if i%s.PerSheet == 0 {
			pages = append(pages, &page{})
		}
		p := pages[len(pages)-1]
		x, y := s.slot(i, off)
		var err error
		if s.Kind == Barcode.Kind {
			err = barcodeLabel(p, x, y, s, l)
		} else {
			spineLabel(p, x, y, s, l)
		}
		if err != nil {
			return nil, fmt.Errorf("label %s: %w", l.Barcode, err)
		}
	}
	if len(pages) == 0 {
		pages = append(pages, &page{})
	}
	return document(fmt.Sprintf("%s labels", s.Name), pages), nil
}

// inset keeps everything printed this far inside a label's edge, so a millimetre or two of
// misalignment still lands on the label.
const inset = 2 * mm

func barcodeLabel(p *page, x, y float64, s Sheet, l Label) error {
	widths, err := Code128(printable(l.Barcode))
	if err != nil {
		return err
	}
	total := modules(widths) + 2*quiet
	// A module of a third of a millimetre or a little less: well over what a phone camera
	// needs, and the whole code with its quiet zones still fits inside the inset.
	module := min(0.33*mm, (s.w-2*inset)/float64(total))
	codeW := float64(total) * module
	barTop, barH := y+1.3*mm, 6.4*mm
	bx := x + (s.w-codeW)/2 + quiet*module
	for i, w := range widths {
		if i%2 == 0 {
			p.rect(bx, barTop, float64(w)*module, barH)
		}
		bx += float64(w) * module
	}
	size := 6.5
	caption := l.Barcode
	if t := strings.TrimSpace(l.Title); t != "" {
		caption += "  " + t
	}
	p.centred(bold, size, x+s.w/2, barTop+barH+2.9*mm, fit(caption, size, s.w-2*inset))
	return nil
}

func spineLabel(p *page, x, y float64, s Sheet, l Label) {
	size, lead := 11.0, 12.5
	lines := l.CallNumber
	if len(lines) > 3 {
		size, lead = 9, 10
	}
	// The block of lines sits in the middle of the label, each line centred; a cap height is
	// about 0.6 em, so the first baseline is that far below the block's top.
	block := lead*float64(len(lines)-1) + 0.6*size
	baseline := y + (s.h-block)/2 + 0.6*size
	for i, line := range lines {
		p.centred(bold, size, x+s.w/2, baseline+float64(i)*lead, fit(line, size, s.w-2*inset))
	}
}

// Alignment draws every label's outline on a sheet, with a cross at each centre and the
// offset in use written in the top margin. Printed on plain paper and held over a sheet of
// labels against a window, it shows which way and how far the printer is off.
func Alignment(s Sheet, off Offset) []byte {
	p := &page{}
	p.text(regular, 8, 0.28125*inch, 0.22*inch,
		fmt.Sprintf("%s alignment, moved %s and %s.", s.Name, along(off.X, "right", "left"), along(off.Y, "down", "up")))
	p.text(regular, 8, 0.28125*inch, 0.38*inch,
		"Print on plain paper at actual size, then hold it over a sheet of labels against the light.")
	for i := range s.PerSheet {
		x, y := s.slot(i, off)
		p.outline(x, y, s.w, s.h, 0)
		p.outline(x+inset, y+inset, s.w-2*inset, s.h-2*inset, 1.5)
		cx, cy := x+s.w/2, y+s.h/2
		p.line(cx-2*mm, cy, cx+2*mm, cy)
		p.line(cx, cy-2*mm, cx, cy+2*mm)
	}
	return document(s.Name+" alignment", []*page{p})
}

func along(mmOff int, plus, minus string) string {
	switch {
	case mmOff > 0:
		return fmt.Sprintf("%d mm %s", mmOff, plus)
	case mmOff < 0:
		return fmt.Sprintf("%d mm %s", -mmOff, minus)
	}
	return "0 mm " + plus
}

// Range is a shelf-edge label: where the shelf is, and the call numbers it runs from and to.
type Range struct {
	Where string // Living room, left · shelf 3
	First string // 823.92 MCD
	Last  string // 891.73 TOL
}

// Ranges draws shelf-edge labels on a sheet: the shelf's place small across the top, then its
// first and last call numbers, large enough to read from a step back.
func Ranges(s Sheet, items []Range, off Offset) []byte {
	var pages []*page
	for i, r := range items {
		if i%s.PerSheet == 0 {
			pages = append(pages, &page{})
		}
		p := pages[len(pages)-1]
		x, y := s.slot(i, off)
		cx, w := x+s.w/2, s.w-2*inset
		p.centred(regular, 6.5, cx, y+inset+1.6*mm, fit(r.Where, 6.5, w))
		size := 9.0
		if r.Last == "" || r.Last == r.First {
			p.centred(bold, size, cx, y+s.h/2+3*mm, fit(r.First, size, w))
			continue
		}
		p.centred(bold, size, cx, y+s.h/2+1.2*mm, fit(r.First, size, w))
		p.centred(bold, size, cx, y+s.h/2+5.2*mm, fit("to "+r.Last, size, w))
	}
	if len(pages) == 0 {
		pages = append(pages, &page{})
	}
	return document(s.Name+" shelf labels", pages)
}
