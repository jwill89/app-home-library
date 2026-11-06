package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// endnoteFormat names EndNote in import refs, field sources, and the report.
var endnoteFormat = importFormat{source: ImportSourceEndNote, name: "EndNote"}

// isXML reports whether a file begins as XML does, as EndNote's export does.
func isXML(first []byte) bool {
	b := bytes.TrimLeft(bytes.TrimPrefix(first, []byte(utf8BOM)), " \t\r\n")
	return bytes.HasPrefix(b, []byte("<?xml")) || bytes.HasPrefix(b, []byte("<xml"))
}

// enText is a field of EndNote's XML: its words, wherever EndNote wrapped them in style runs.
type enText string

// UnmarshalXML keeps the text of the element and of everything in it.
func (t *enText) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var b strings.Builder
	for depth := 0; ; {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch v := tok.(type) {
		case xml.CharData:
			b.Write(v)
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if depth == 0 {
				*t = enText(b.String())
				return nil
			}
			depth--
		}
	}
}

// lines are a field's values: EndNote keeps several in one field, a line each.
func (t enText) lines() []string {
	var out []string
	for _, l := range strings.FieldsFunc(string(t), func(r rune) bool { return r == '\r' || r == '\n' }) {
		if l = strings.Join(strings.Fields(l), " "); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// first is a field's first value, without the punctuation a catalog leaves after it.
func (t enText) first() string {
	if l := t.lines(); len(l) > 0 {
		return strings.TrimRight(l[0], " ;:,/")
	}
	return ""
}

// text is a field's values as paragraphs.
func (t enText) text() string {
	return strings.Join(t.lines(), "\n")
}

// enRecord is one reference in EndNote's XML export, the fields the library has a place for.
type enRecord struct {
	RecNumber string `xml:"rec-number"`
	Key       struct {
		DBID string `xml:"db-id,attr"`
	} `xml:"foreign-keys>key"`
	RefType struct {
		Name string `xml:"name,attr"`
	} `xml:"ref-type"`
	Authors        []enText `xml:"contributors>authors>author"`
	Editors        []enText `xml:"contributors>secondary-authors>author"`
	Translators    []enText `xml:"contributors>translated-authors>author"`
	Title          enText   `xml:"titles>title"`
	SecondaryTitle enText   `xml:"titles>secondary-title"`
	Periodical     enText   `xml:"periodical>full-title"`
	Pages          enText   `xml:"pages"`
	Volume         enText   `xml:"volume"`
	Number         enText   `xml:"number"`
	Edition        enText   `xml:"edition"`
	Keywords       []enText `xml:"keywords>keyword"`
	Year           enText   `xml:"dates>year"`
	PubLocation    enText   `xml:"pub-location"`
	Publisher      enText   `xml:"publisher"`
	ISBN           enText   `xml:"isbn"`
	AccessionNum   enText   `xml:"accession-num"`
	CallNum        enText   `xml:"call-num"`
	DOI            enText   `xml:"electronic-resource-num"`
	URLs           []enText `xml:"urls>related-urls>url"`
	Abstract       enText   `xml:"abstract"`
	Notes          enText   `xml:"notes"`
	ResearchNotes  enText   `xml:"research-notes"`
	Language       enText   `xml:"language"`
}

// ref is the record's id, unique to its EndNote library.
func (r *enRecord) ref() string {
	if r.Key.DBID != "" {
		return r.Key.DBID + "/" + r.RecNumber
	}
	return r.RecNumber
}

// kind is what the record becomes: a book, or a document of a kind.
func (r *enRecord) kind() string {
	switch r.RefType.Name {
	case "Book", "Edited Book", "Electronic Book", "Classical Work":
		return "book"
	case "Book Section", "Electronic Book Section":
		return "chapter"
	case "Journal Article", "Magazine Article", "Newspaper Article", "Electronic Article", "Conference Paper", "Conference Proceedings":
		return "article"
	}
	return "other"
}

// notes are the owner's notes. A record EndNote took from a library catalog keeps the catalog's
// accession number, and its Notes are the catalog's (record number, size, bibliography), so only
// its research notes come in; the lookup brings what the library keeps of the rest.
func (r *enRecord) notes() string {
	if r.AccessionNum.first() != "" {
		return r.ResearchNotes.text()
	}
	return joinNotes(r.Notes.text(), r.ResearchNotes.text())
}

// subjects are the record's keywords, without the full stop a catalog heading ends with.
func (r *enRecord) subjects() []string {
	var out []string
	for _, k := range r.Keywords {
		for _, l := range k.lines() {
			out = append(out, strings.TrimSuffix(l, "."))
		}
	}
	return cleanList(out)
}

var (
	bookPagesRe = regexp.MustCompile(`(\d+)\s*p\b`)
	// A call number as the Library of Congress writes it, after the reading room's name:
	// "QC174.12; .A42 2003".
	callNumberRe = regexp.MustCompile(`\b([A-Z]{1,3}\s?\d{1,4}(?:\.\d+)?)[;\s]+(\.?[A-Z]\d+.*)$`)
)

// year is the first year in a field: "c1998" and "[1985]" are years.
func year(t enText) string {
	return lookup.Year(string(t))
}

// bookPages is a book's page count from a catalog's extent: "xx, 711 p., 1 leaf of plates".
func bookPages(t enText) int64 {
	s := t.first()
	if m := bookPagesRe.FindAllStringSubmatch(s, -1); m != nil {
		n, _ := strconv.ParseInt(m[len(m)-1][1], 10, 64)
		return n
	}
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// isbnAndFormat is the first ISBN a field holds, and the binding the catalog wrote beside it.
func isbnAndFormat(t enText) (isbn, format string) {
	for _, l := range t.lines() {
		for _, tok := range strings.Fields(l) {
			n, ok := lookup.NormalizeISBN(strings.Trim(tok, "()[];:,"))
			if !ok {
				continue
			}
			q := strings.ToLower(l)
			switch {
			case strings.Contains(q, "pbk"), strings.Contains(q, "paperback"):
				format = "paperback"
			case strings.Contains(q, "hbk"), strings.Contains(q, "hardback"), strings.Contains(q, "cloth"), strings.Contains(q, "hardcover"):
				format = "hardcover"
			}
			return n, format
		}
	}
	return "", ""
}

// callNumber is the first Library of Congress call number a field holds, as the catalog writes it.
func callNumber(t enText) string {
	for _, l := range t.lines() {
		if m := callNumberRe.FindStringSubmatch(l); m != nil {
			return m[1] + " " + strings.Join(strings.Fields(strings.ReplaceAll(m[2], ";", "")), " ")
		}
	}
	return ""
}

// person turns a catalog's "Last, First, dates" into the name as it is read: "First Last", as
// every source's heading is turned.
func person(t enText) string {
	return lookup.InvertName(strings.Join(strings.Fields(string(t)), " "))
}

// The short words a title leaves in lower case, but first or last.
var minorWords = map[string]bool{
	"a": true, "an": true, "and": true, "as": true, "at": true, "but": true, "by": true, "for": true,
	"from": true, "in": true, "into": true, "nor": true, "of": true, "on": true, "or": true, "over": true,
	"per": true, "the": true, "to": true, "up": true, "via": true, "vs": true, "with": true,
}

// catalogTitle splits a catalog's "Title : subtitle", and capitalizes an English one the catalog
// wrote in sentence case as its cover does: "Revolution in science" is "Revolution in Science".
func catalogTitle(s, language string) (title, subtitle string) {
	s = strings.TrimRight(strings.Join(strings.Fields(s), " "), " ./;:")
	title, subtitle, _ = strings.Cut(s, ":")
	title, subtitle = strings.TrimSpace(title), strings.TrimSpace(subtitle)
	if language != "" && language != "en" {
		return title, subtitle
	}
	return headline(title), headline(subtitle)
}

// headline capitalizes each word of a title in sentence case, but the short ones inside it; a
// word with capitals of its own (DNA, iPhone, McDonald) is left as it is.
func headline(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w != strings.ToLower(w) || (i > 0 && i < len(words)-1 && minorWords[w]) {
			continue
		}
		for j, r := range w {
			if unicode.IsLetter(r) {
				words[i] = w[:j] + string(unicode.ToUpper(r)) + w[j+utf8.RuneLen(r):]
				break
			}
		}
	}
	return strings.Join(words, " ")
}

// endnoteBook reads one of EndNote's books. EndNote does not say which books the owner has, so
// none is marked owned: the owner says where they go. The subjects of a book with an ISBN come
// from the lookup, as the catalog writes them; EndNote's own keep only the words.
func endnoteBook(r *enRecord) *importedBook {
	lang := lookup.NormalizeLanguage(r.Language.first())
	in := BookInput{
		ReadStatus:       "unread",
		Publisher:        r.Publisher.first(),
		PublishPlace:     r.PubLocation.first(),
		Published:        year(r.Year),
		EditionStatement: r.Edition.first(),
		Pages:            bookPages(r.Pages),
		Language:         lang,
		LCC:              callNumber(r.CallNum),
		Description:      r.Abstract.text(),
		Notes:            r.notes(),
		Series:           r.SecondaryTitle.first(),
		Sources:          map[string]string{},
	}
	in.Title, in.Subtitle = catalogTitle(string(r.Title), lang)
	in.ISBN, in.Format = isbnAndFormat(r.ISBN)
	if in.Series != "" {
		in.SeriesNumber = r.Volume.first()
	}
	role := "author"
	if r.RefType.Name == "Edited Book" {
		role = "editor"
	}
	for _, a := range r.Authors {
		if name := person(a); name != "" {
			in.Contributors = append(in.Contributors, lookup.Contributor{Name: name, Role: role})
		}
	}
	for _, a := range r.Translators {
		if name := person(a); name != "" {
			in.Contributors = append(in.Contributors, lookup.Contributor{Name: name, Role: "translator"})
		}
	}
	if in.ISBN == "" {
		in.Subjects = r.subjects()
	}
	imported(&in, ImportSourceEndNote)
	return &importedBook{
		Ref:    r.ref(),
		In:     in,
		First:  CopyInput{AcquiredAs: "unknown", PriceCents: -1, Condition: "unknown", DustJacket: "unknown"},
		Copies: 1,
	}
}

// endnoteDocument reads one of EndNote's articles, chapters, or other papers, as it wrote it.
func endnoteDocument(r *enRecord, kind string) DocumentInput {
	in := DocumentInput{
		Kind:      kind,
		Title:     strings.TrimSuffix(strings.Join(strings.Fields(string(r.Title)), " "), "."),
		Authors:   []string{},
		Container: r.SecondaryTitle.first(),
		Volume:    r.Volume.first(),
		Issue:     r.Number.first(),
		Pages:     r.Pages.first(),
		Published: year(r.Year),
		Publisher: r.Publisher.first(),
		Notes:     r.notes(),
		Subjects:  r.subjects(),
	}
	if in.Container == "" {
		in.Container = r.Periodical.first()
	}
	for _, a := range r.Authors {
		if name := person(a); name != "" {
			in.Authors = append(in.Authors, name)
		}
	}
	// A section's secondary authors are its book's editors; an article's are not cited.
	if kind == "chapter" {
		in.Editors = []string{}
		for _, a := range r.Editors {
			if name := person(a); name != "" {
				in.Editors = append(in.Editors, name)
			}
		}
	}
	if doi, ok := lookup.NormalizeDOI(r.DOI.first()); ok {
		in.DOI = doi
	}
	for _, u := range r.URLs {
		if l := u.first(); strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://") {
			in.URL = l
			break
		}
	}
	in.Abstract = r.Abstract.text()
	return in
}

// importEndNote reads EndNote's XML export (File, Export, as XML): its books join the library as
// the other apps' do, and its articles, chapters, and other papers become documents.
func (s *Service) importEndNote(ctx context.Context, r io.Reader, o ImportOptions) (*ImportReport, error) {
	rep := &ImportReport{Source: endnoteFormat.name, Problems: []string{}}
	var queue []int64
	d := xml.NewDecoder(r)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if rep.Rows == 0 {
				return nil, &FieldError{"file", "this XML could not be read: " + err.Error()}
			}
			rep.Problems = append(rep.Problems, fmt.Sprintf("after record %d the file could not be read: %v", rep.Rows, err))
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "record" {
			continue
		}
		var rec enRecord
		if err := d.DecodeElement(&rec, &se); err != nil {
			return rep, fmt.Errorf("record %d: %w", rep.Rows+1, err)
		}
		rep.Rows++
		where := "record " + rec.RecNumber
		kind := rec.kind()
		if kind == "book" {
			b := endnoteBook(&rec)
			id, outcome, err := s.importBook(ctx, &endnoteFormat, b, o)
			queue = rep.tally(where, b, id, outcome, err, queue)
			continue
		}
		in := endnoteDocument(&rec, kind)
		switch outcome, err := s.importDocument(ctx, rec.ref(), in); {
		case err != nil:
			rep.Problems = append(rep.Problems, fmt.Sprintf("%s (%s): %v", where, in.Title, err))
		case outcome == importAlready:
			rep.Already++
		case outcome == importFilled:
			rep.Filled++
		case outcome == importMatched:
			rep.DocumentsHad++
		default:
			rep.Documents++
		}
	}
	if rep.Rows == 0 {
		return nil, &FieldError{"file", "this XML has no EndNote references: in EndNote, select the references, then choose File, Export, and save as XML"}
	}
	return rep, s.lookUpImported(ctx, rep, queue)
}

// importDocument brings in one of EndNote's papers as a document. One imported before, or one
// whose DOI the library has, is only linked to its record, so a second import adds nothing.
func (s *Service) importDocument(ctx context.Context, ref string, in DocumentInput) (importOutcome, error) {
	if err := in.normalize(); err != nil {
		return 0, err
	}
	outcome := importAdded
	var had int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		refID, err := q.GetDocumentImportRef(ctx, db.GetDocumentImportRefParams{Source: ImportSourceEndNote, ExternalID: ref})
		if err == nil {
			outcome, had = importAlready, refID
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var id int64
		if in.DOI != "" {
			if id, err = q.DocumentByDOI(ctx, in.DOI); err == nil {
				outcome, had = importMatched, id
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if outcome != importMatched {
			if err := checkDocument(ctx, q, 0, in); err != nil {
				return err
			}
			if id, err = q.CreateDocument(ctx, in.params()); err != nil {
				return fmt.Errorf("create document: %w", err)
			}
			if err := index(ctx, q, id); err != nil {
				return err
			}
		}
		return q.CreateDocumentImportRef(ctx, db.CreateDocumentImportRefParams{Source: ImportSourceEndNote, ExternalID: ref, DocumentID: id})
	})
	if err != nil || had == 0 {
		return outcome, err
	}
	// Here already: its blanks are filled from the record.
	changed, err := s.fillDocument(ctx, had, in)
	if err == nil && changed && outcome == importAlready {
		outcome = importFilled
	}
	return outcome, err
}

// fillDocument fills a document's blanks from an imported record that names it, and says
// whether anything changed.
func (s *Service) fillDocument(ctx context.Context, id int64, from DocumentInput) (bool, error) {
	d, err := s.GetDocument(ctx, id)
	if err != nil {
		return false, err
	}
	have := d.DocumentInput
	if len(fillDocumentBlanks(&have, from)) == 0 {
		return false, nil
	}
	_, err = s.UpdateDocument(ctx, id, have)
	return err == nil, err
}
