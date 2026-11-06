package lookup

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// LoCURL is the Library of Congress catalog's SRU endpoint. It answers MARC records, free
// and without a key, and only over plain HTTP.
const LoCURL = "http://lx2.loc.gov:210/lcdb"

// LoC asks the Library of Congress catalog.
type LoC struct {
	BaseURL string
	fetch   *fetcher
}

func (l *LoC) query(ctx context.Context, cql string, limit int) ([]byte, error) {
	q := url.Values{
		"version":        {"1.1"},
		"operation":      {"searchRetrieve"},
		"query":          {cql},
		"maximumRecords": {strconv.Itoa(limit)},
		"recordSchema":   {"marcxml"},
	}
	return l.fetch.get(ctx, l.BaseURL+"?"+q.Encode())
}

// FetchISBN returns the raw answer for one ISBN.
func (l *LoC) FetchISBN(ctx context.Context, isbn string) ([]byte, error) {
	return l.query(ctx, "bath.isbn="+isbn, 1)
}

// FetchTitle returns the raw answer for a title and author, which finds the editions the
// catalog has when it lacks the one in hand: a paperback reprint, as a rule.
func (l *LoC) FetchTitle(ctx context.Context, title, author string) ([]byte, error) {
	cql := fmt.Sprintf("dc.title=%q", strings.ReplaceAll(title, `"`, ""))
	if author != "" {
		cql += fmt.Sprintf(" and dc.creator=%q", strings.ReplaceAll(author, `"`, ""))
	}
	return l.query(ctx, cql, 5)
}

type marcCollection struct {
	Records []marcRecord `xml:"records>record>recordData>record"`
}

type marcRecord struct {
	Controls []struct {
		Tag   string `xml:"tag,attr"`
		Value string `xml:",chardata"`
	} `xml:"controlfield"`
	Fields []marcField `xml:"datafield"`
}

type marcField struct {
	Tag  string `xml:"tag,attr"`
	Ind1 string `xml:"ind1,attr"`
	Ind2 string `xml:"ind2,attr"`
	Subs []struct {
		Code  string `xml:"code,attr"`
		Value string `xml:",chardata"`
	} `xml:"subfield"`
}

func (f marcField) all(code string) []string {
	var out []string
	for _, s := range f.Subs {
		if s.Code == code {
			out = append(out, s.Value)
		}
	}
	return out
}

func (f marcField) first(code string) string {
	if v := f.all(code); len(v) > 0 {
		return v[0]
	}
	return ""
}

func (m marcRecord) fields(tag string) []marcField {
	var out []marcField
	for _, f := range m.Fields {
		if f.Tag == tag {
			out = append(out, f)
		}
	}
	return out
}

func (m marcRecord) field(tag string) (marcField, bool) {
	if f := m.fields(tag); len(f) > 0 {
		return f[0], true
	}
	return marcField{}, false
}

// ParseLoC reads every MARC record in an SRU answer. isbn, when given, picks the binding and
// volume from the 020 line for that ISBN: one record often lists a series' every volume.
func ParseLoC(body []byte, isbn string) ([]*Record, error) {
	var c marcCollection
	if err := xml.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("loc: %w", err)
	}
	out := make([]*Record, 0, len(c.Records))
	for _, m := range c.Records {
		out = append(out, m.record(isbn))
	}
	return out, nil
}

var (
	pagesRe  = regexp.MustCompile(`(\d+)\s*(?:p\b|pages|leaves)`)
	cmRe     = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:x\s*(\d+(?:\.\d+)?)\s*)?cm`)
	volumeRe = regexp.MustCompile(`v\.\s*(\d+(?:\.\d+)?)`)
)

func (m marcRecord) record(isbn string) *Record {
	r := &Record{}
	if f, ok := m.field("245"); ok {
		r.Title = cleanField(f.first("a"))
		if n := f.first("n"); n != "" {
			r.Title += ", " + cleanField(n)
		}
		if p := f.first("p"); p != "" {
			r.Title += ". " + cleanField(p)
		}
		r.Subtitle = cleanField(f.first("b"))
	}
	if f, ok := m.field("100"); ok {
		r.Contributors = append(r.Contributors, Contributor{Name: InvertName(f.first("a")), Role: "author"})
	}
	for _, f := range m.fields("700") {
		if f.first("t") != "" {
			continue // a work inside this one, not a person who made it
		}
		r.Contributors = append(r.Contributors, Contributor{Name: InvertName(f.first("a")), Role: marcRole(strings.Join(f.all("e"), " "))})
	}
	pub, ok := m.publication()
	if ok {
		r.Place = cleanField(pub.first("a"))
		r.Publisher = cleanField(pub.first("b"))
		r.Published = NormalizeDate(pub.first("c"))
	}
	if f, ok := m.field("250"); ok {
		r.Edition = cleanField(f.first("a"))
	}
	if f, ok := m.field("300"); ok {
		for _, p := range pagesRe.FindAllStringSubmatch(f.first("a"), -1) {
			if n, _ := strconv.Atoi(p[1]); n > r.Pages {
				r.Pages = n
			}
		}
		if dim := cmRe.FindStringSubmatch(f.first("c")); dim != nil {
			h, _ := strconv.ParseFloat(dim[1], 64)
			r.HeightMM = lengthMM(h, "cm")
			if dim[2] != "" {
				w, _ := strconv.ParseFloat(dim[2], 64)
				r.WidthMM = lengthMM(w, "cm")
			}
		}
	}
	for _, f := range m.fields("020") {
		a := f.first("a")
		if isbn != "" && !strings.Contains(strings.ReplaceAll(a, "-", ""), isbn) && !strings.Contains(a, ISBN10(isbn)) {
			continue
		}
		q := strings.Join(f.all("q"), " ") + " " + a
		if r.Format == "" {
			r.Format = FormatFromText(q)
		}
		if v := volumeRe.FindStringSubmatch(q); v != nil && r.SeriesNumber == "" {
			r.SeriesNumber = v[1]
		}
		if isbn != "" {
			break
		}
	}
	for _, f := range m.fields("082") {
		if d := NormalizeDDC(f.first("a")); d != "" {
			r.DDC = d
			break
		}
	}
	if f, ok := m.field("050"); ok {
		r.LCC = strings.TrimSpace(strings.Join(f.all("a"), " ") + " " + f.first("b"))
	}
	if f, ok := m.field("010"); ok {
		r.LCCN = strings.TrimSpace(f.first("a"))
	}
	for _, tag := range []string{"650", "600", "651", "655"} {
		for _, f := range m.fields(tag) {
			if tag == "655" && f.first("2") != "lcgft" {
				continue
			}
			parts := []string{cleanField(f.first("a"))}
			for _, code := range []string{"x", "z", "v"} {
				for _, s := range f.all(code) {
					parts = append(parts, cleanField(s))
				}
			}
			r.Subjects = appendUnique(r.Subjects, strings.Join(parts, " -- "))
		}
	}
	series := m.fields("830")
	if len(series) == 0 {
		series = m.fields("490")
	}
	if len(series) > 0 {
		r.Series = cleanField(series[0].first("a"))
		if v := series[0].first("v"); v != "" && r.SeriesNumber == "" {
			r.SeriesNumber = readNumber(v)
		}
	}
	for _, c := range m.Controls {
		if c.Tag == "008" && len(c.Value) >= 38 {
			r.Language = NormalizeLanguage(c.Value[35:38])
		}
	}
	return r
}

// publication is 264 with second indicator 1 (publication), or the older 260.
func (m marcRecord) publication() (marcField, bool) {
	for _, f := range m.fields("264") {
		if f.Ind2 == "1" {
			return f, true
		}
	}
	return m.field("260")
}

func marcRole(e string) string {
	e = strings.ToLower(e)
	switch {
	case strings.Contains(e, "translat"):
		return "translator"
	case strings.Contains(e, "illustrat"), strings.Contains(e, "artist"):
		return "illustrator"
	case strings.Contains(e, "editor"):
		return "editor"
	case strings.Contains(e, "narrat"):
		return "narrator"
	case strings.Contains(e, "author"), strings.Contains(e, "writer"):
		return "author"
	}
	return "contributor"
}

func appendUnique(list []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return list
	}
	for _, s := range list {
		if strings.EqualFold(s, v) {
			return list
		}
	}
	return append(list, v)
}
