package labels

import (
	"bytes"
	"fmt"
	"strings"
)

// A page is drawn with a few PDF operators on a US Letter page, in points from the top left;
// the writer turns them into PDF's own coordinates, which start at the bottom left. The only
// fonts are Courier and Courier-Bold, which every PDF reader has, so nothing is embedded and
// every character is 0.6 em wide, which is how text is centred and cut to fit.

const (
	pageW = 612.0 // US Letter, in points
	pageH = 792.0
	mm    = 72 / 25.4
	inch  = 72.0
	// charW is a Courier character's width, in ems.
	charW = 0.6
)

type font string

const (
	regular font = "/F1"
	bold    font = "/F2"
)

type page struct{ b strings.Builder }

func (p *page) op(format string, args ...any) {
	fmt.Fprintf(&p.b, format, args...)
	p.b.WriteByte('\n')
}

// rect fills a rectangle whose top left corner is at x, y.
func (p *page) rect(x, y, w, h float64) {
	p.op("%.3f %.3f %.3f %.3f re f", x, pageH-y-h, w, h)
}

// outline strokes a rectangle, dashed when dash > 0.
func (p *page) outline(x, y, w, h, dash float64) {
	if dash > 0 {
		p.op("[%.2f %.2f] 0 d", dash, dash)
	}
	p.op("0.4 w %.3f %.3f %.3f %.3f re S [] 0 d", x, pageH-y-h, w, h)
}

func (p *page) line(x1, y1, x2, y2 float64) {
	p.op("0.4 w %.3f %.3f m %.3f %.3f l S", x1, pageH-y1, x2, pageH-y2)
}

// text writes s with its baseline at y, starting at x.
func (p *page) text(f font, size, x, y float64, s string) {
	p.op("BT %s %.2f Tf %.3f %.3f Td (%s) Tj ET", f, size, x, pageH-y, pdfString(s))
}

// centred writes s centred on cx.
func (p *page) centred(f font, size, cx, y float64, s string) {
	p.text(f, size, cx-width(s, size)/2, y, s)
}

// width is how wide s is in Courier at size, in points.
func width(s string, size float64) float64 {
	return float64(len([]rune(s))) * charW * size
}

// fit cuts s to as many characters as fit in w points at size, marking a cut with an ellipsis.
func fit(s string, size, w float64) string {
	n := int(w / (charW * size))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 2 {
		return ""
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// winAnsi is the WinAnsiEncoding byte for each character outside ASCII that a title is likely
// to have; anything else prints as a question mark.
var winAnsi = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88,
	'‰': 0x89, 'Š': 0x8a, '‹': 0x8b, 'Œ': 0x8c, 'Ž': 0x8e, '‘': 0x91, '’': 0x92, '“': 0x93,
	'”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97, '˜': 0x98, '™': 0x99, 'š': 0x9a, '›': 0x9b,
	'œ': 0x9c, 'ž': 0x9e, 'Ÿ': 0x9f,
}

// pdfString escapes s for a PDF literal string in WinAnsiEncoding.
func pdfString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r >= 32 && r < 127:
			b.WriteRune(r)
		case r >= 0xa0 && r <= 0xff:
			fmt.Fprintf(&b, "\\%03o", r)
		default:
			if c, ok := winAnsi[r]; ok {
				fmt.Fprintf(&b, "\\%03o", c)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// document writes the pages as one PDF file.
func document(title string, pages []*page) []byte {
	var out bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	// Objects 1 to 5: the catalog, the page tree, the two fonts, the document's title. Each
	// page is then a page object and its content stream.
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 6+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R /ViewerPreferences << /PrintScaling /None >> >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages)))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>")
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Bold /Encoding /WinAnsiEncoding >>")
	obj(fmt.Sprintf("<< /Title (%s) /Producer (Home Library) >>", pdfString(title)))
	for i, p := range pages {
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] "+
			"/Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>", pageW, pageH, 7+2*i))
		content := p.b.String()
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content))
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return out.Bytes()
}
