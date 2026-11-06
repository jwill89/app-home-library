package catalog

import (
	"context"
	"slices"
	"strings"
	"testing"

	"homelibrary/internal/lookup"
)

// An EndNote XML export as EndNote writes it: every field in style runs, several values to a
// field on their own lines (&#xD;). The first record is one of the owner's, as exported.
const endnoteXML = `<?xml version="1.0" encoding="UTF-8" ?><xml><records>` +
	`<record><database name="HPS Library.enl" path="C:\HPS Library.enl">HPS Library.enl</database><source-app name="EndNote" version="19.0">EndNote</source-app><rec-number>87</rec-number><foreign-keys><key app="EN" db-id="xesfez">87</key></foreign-keys><ref-type name="Book">6</ref-type><contributors><authors><author><style face="normal" font="default" size="100%">Al-Khalili, Jim</style></author></authors></contributors><titles><title><style face="normal" font="default" size="100%">Quantum</style></title></titles><pages><style face="normal" font="default" size="100%">280 p.</style></pages><keywords><keyword><style face="normal" font="default" size="100%">Quantum theory Popular works.</style></keyword></keywords><dates><year><style face="normal" font="default" size="100%">2003</style></year></dates><pub-location><style face="normal" font="default" size="100%">London&#xD;New York</style></pub-location><publisher><style face="normal" font="default" size="100%">Weidenfeld &amp; Nicolson ;&#xD;Sterling</style></publisher><isbn><style face="normal" font="default" size="100%">0297843052</style></isbn><accession-num><style face="normal" font="default" size="100%">13381200</style></accession-num><call-num><style face="normal" font="default" size="100%">Reference - Science Reading Room (Adams, 5th Floor) QC174.12; .A42 2003&#xD;Jefferson or Adams Building Reading Rooms QC174.12; .A42 2003</style></call-num><notes><style face="normal" font="default" size="100%">2003274940&#xD;col. ill. ; 23 cm.</style></notes></record>` +
	`<record><rec-number>88</rec-number><foreign-keys><key app="EN" db-id="xesfez">88</key></foreign-keys><ref-type name="Edited Book">28</ref-type><contributors><authors><author><style face="normal" font="default" size="100%">Kourany, Janet A.</style></author></authors></contributors><titles><title><style face="normal" font="default" size="100%">Scientific knowledge : basic issues in the philosophy of science</style></title></titles><pages><style face="normal" font="default" size="100%">viii, 440 p.</style></pages><keywords><keyword><style face="normal" font="default" size="100%">Science Philosophy.</style></keyword></keywords><dates><year><style face="normal" font="default" size="100%">c1998</style></year></dates><research-notes><style face="normal" font="default" size="100%">Chapter 3 first.</style></research-notes></record>` +
	`<record><rec-number>89</rec-number><foreign-keys><key app="EN" db-id="xesfez">89</key></foreign-keys><ref-type name="Journal Article">17</ref-type><contributors><authors><author><style face="normal" font="default" size="100%">Ruiz, Ana María</style></author><author><style face="normal" font="default" size="100%">Darwin, Charles, 1809-1882</style></author></authors></contributors><titles><title><style face="normal" font="default" size="100%">Teaching calculus for understanding</style></title><secondary-title><style face="normal" font="default" size="100%">The Journal of Higher Education</style></secondary-title></titles><pages><style face="normal" font="default" size="100%">623–649</style></pages><volume><style face="normal" font="default" size="100%">70</style></volume><number><style face="normal" font="default" size="100%">6</style></number><keywords><keyword><style face="normal" font="default" size="100%">calculus</style></keyword></keywords><dates><year><style face="normal" font="default" size="100%">1999</style></year></dates><electronic-resource-num><style face="normal" font="default" size="100%">https://doi.org/10.1080/00221546.1999.11780773</style></electronic-resource-num><abstract><style face="normal" font="default" size="100%">A study of first-year students.</style></abstract><urls><pdf-urls><url>internal-pdf://0001/Ruiz.pdf</url></pdf-urls><related-urls><url><style face="normal" font="default" size="100%">https://example.org/ruiz</style></url></related-urls></urls></record>` +
	`<record><rec-number>90</rec-number><foreign-keys><key app="EN" db-id="xesfez">90</key></foreign-keys><ref-type name="Book Section">5</ref-type><contributors><authors><author><style face="normal" font="default" size="100%">Norton, John D.</style></author></authors><secondary-authors><author><style face="normal" font="default" size="100%">Price, Huw</style></author><author><style face="normal" font="default" size="100%">Corry, Richard</style></author></secondary-authors></contributors><titles><title><style face="normal" font="default" size="100%">Causation as folk science</style></title><secondary-title><style face="normal" font="default" size="100%">Causation, Physics, and the Constitution of Reality</style></secondary-title></titles><dates><year><style face="normal" font="default" size="100%">2007</style></year></dates><publisher><style face="normal" font="default" size="100%">Oxford University Press</style></publisher></record>` +
	`</records></xml>`

func TestImportEndNote(t *testing.T) {
	ctx := context.Background()
	s := newService(t)

	rep, err := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "library"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Source != "EndNote" || rep.Rows != 4 || rep.Added != 2 || rep.Documents != 2 || rep.NoISBN != 1 || rep.Queued != 1 || len(rep.Problems) != 0 {
		t.Fatalf("report %+v", rep)
	}

	// A book from a catalog: the name as read, the first place and publisher, the call number;
	// the catalog's notes and flattened headings left to the lookup.
	isbn, _ := lookup.NormalizeISBN("0297843052")
	id, err := s.FindByISBN(ctx, isbn)
	if err != nil || id == 0 {
		t.Fatalf("Quantum not added: %v", err)
	}
	q, _ := s.Get(ctx, id)
	if q.Title != "Quantum" || q.Publisher != "Weidenfeld & Nicolson" || q.PublishPlace != "London" || q.Published != "2003" ||
		q.Pages != 280 || q.LCC != "QC174.12 .A42 2003" || q.Notes != "" || len(q.Subjects) != 0 || len(q.Copies) != 1 {
		t.Fatalf("Quantum %+v", q)
	}
	if len(q.Contributors) != 1 || q.Contributors[0] != (lookup.Contributor{Name: "Jim Al-Khalili", Role: "author"}) || q.Sources["title"] != "endnote" {
		t.Fatalf("Quantum people %v, sources %v", q.Contributors, q.Sources)
	}

	// A book without an ISBN keeps EndNote's keywords and the owner's notes; its catalog title
	// is split and capitalized.
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var edited *Book
	for _, r := range rows {
		if strings.HasPrefix(r.Title, "Scientific") {
			edited, _ = s.Get(ctx, r.ID)
		}
	}
	if edited == nil || edited.Title != "Scientific Knowledge" || edited.Subtitle != "Basic Issues in the Philosophy of Science" ||
		edited.Published != "1998" || edited.Pages != 440 || edited.Notes != "Chapter 3 first." ||
		!slices.Equal(edited.Subjects, []string{"Science Philosophy"}) || edited.Contributors[0].Role != "editor" {
		t.Fatalf("edited book %+v", edited)
	}

	// The article and the chapter are documents.
	docs, err := s.Documents(ctx, "", nil)
	if err != nil || len(docs) != 2 {
		t.Fatalf("documents %+v, %v", docs, err)
	}
	for _, row := range docs {
		d, _ := s.GetDocument(ctx, row.ID)
		switch d.Kind {
		case "article":
			if d.Title != "Teaching calculus for understanding" || d.DOI != "10.1080/00221546.1999.11780773" ||
				d.Container != "The Journal of Higher Education" || d.Issue != "6" || d.Pages != "623-649" ||
				d.URL != "https://example.org/ruiz" || !slices.Equal(d.Authors, []string{"Ana María Ruiz", "Charles Darwin"}) ||
				!slices.Equal(d.Subjects, []string{"calculus"}) || !strings.HasPrefix(d.Abstract, "A study") || strings.Contains(d.Notes, "Abstract") {
				t.Errorf("article %+v", d)
			}
		case "chapter":
			if d.Container != "Causation, Physics, and the Constitution of Reality" || d.Publisher != "Oxford University Press" ||
				!slices.Equal(d.Authors, []string{"John D. Norton"}) || !slices.Equal(d.Editors, []string{"Huw Price", "Richard Corry"}) {
				t.Errorf("chapter %+v", d)
			}
		default:
			t.Errorf("a document of kind %q", d.Kind)
		}
	}

	// Imported again, nothing is added.
	again, err := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "library"})
	if err != nil || again.Already != 4 || again.Added != 0 || again.Documents != 0 {
		t.Fatalf("again %+v, %v", again, err)
	}
}

func TestImportEndNoteNotOwned(t *testing.T) {
	ctx := context.Background()

	// Left out: the books stay out, and the papers still come in.
	s := newService(t)
	rep, err := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "leave"})
	if err != nil || rep.Added != 0 || rep.NotOwned != 2 || rep.Documents != 2 {
		t.Fatalf("left out %+v, %v", rep, err)
	}

	// On the wishlist: only the book with an ISBN, which a second import can match. A paper the
	// library had by its DOI is linked, not added again.
	s = newService(t)
	if _, err := s.CreateDocument(ctx, DocumentInput{Title: "Teaching Calculus", DOI: "10.1080/00221546.1999.11780773"}); err != nil {
		t.Fatal(err)
	}
	rep, err = s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "wishlist"})
	if err != nil || rep.Wanted != 1 || rep.NotOwned != 1 || rep.Documents != 1 || rep.DocumentsHad != 1 {
		t.Fatalf("wishlist %+v, %v", rep, err)
	}
	if _, err := s.ImportFile(ctx, strings.NewReader("<?xml version=\"1.0\"?><xml><records></records></xml>"), ImportOptions{}); err == nil {
		t.Error("an export with no references was taken")
	}
}

func TestEndNoteNames(t *testing.T) {
	for in, want := range map[string]string{
		"Al-Khalili, Jim":            "Jim Al-Khalili",
		"Darwin, Charles, 1809-1882": "Charles Darwin",
		"King, Martin Luther, Jr.":   "Martin Luther King, Jr.",
		"World Health Organization":  "World Health Organization",
	} {
		if got := person(enText(in)); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string][2]string{
		"Revolution in science": {"Revolution in Science", ""},
		"Darwin's dangerous idea : evolution and the meanings of life": {"Darwin's Dangerous Idea", "Evolution and the Meanings of Life"},
		"DNA: the secret of life":                                      {"DNA", "The Secret of Life"},
	} {
		if title, sub := catalogTitle(in, ""); title != want[0] || sub != want[1] {
			t.Errorf("%q: %q / %q", in, title, sub)
		}
	}
	if title, _ := catalogTitle("La structure des révolutions scientifiques", "fr"); title != "La structure des révolutions scientifiques" {
		t.Errorf("a French title was capitalized: %q", title)
	}
}

func TestReimportFillsBlanks(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "library"}); err != nil {
		t.Fatal(err)
	}
	// Since the first import, the book lost its publisher (as a lookup might leave it), and the
	// owner emptied its place of publication; the article lost its abstract.
	isbn, _ := lookup.NormalizeISBN("0297843052")
	id, _ := s.FindByISBN(ctx, isbn)
	b, _ := s.Get(ctx, id)
	in := b.Input()
	in.Publisher, in.Sources["publisher"] = "", ""
	in.PublishPlace, in.Sources["publishPlace"] = "", SourceOwner
	if err := s.Update(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	docs, _ := s.Documents(ctx, "", nil)
	var article *Document
	for _, row := range docs {
		if d, _ := s.GetDocument(ctx, row.ID); d.Kind == "article" {
			article = d
		}
	}
	cut := article.DocumentInput
	cut.Abstract = ""
	if _, err := s.UpdateDocument(ctx, article.ID, cut); err != nil {
		t.Fatal(err)
	}

	rep, err := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "library"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Filled != 2 || rep.Already != 2 || rep.Added != 0 || rep.Documents != 0 {
		t.Fatalf("report %+v", rep)
	}
	if b, _ = s.Get(ctx, id); b.Publisher == "" || b.Sources["publisher"] != ImportSourceEndNote || b.PublishPlace != "" {
		t.Errorf("book: publisher %q from %q, place %q", b.Publisher, b.Sources["publisher"], b.PublishPlace)
	}
	if d, _ := s.GetDocument(ctx, article.ID); !strings.HasPrefix(d.Abstract, "A study") {
		t.Errorf("abstract %q", d.Abstract)
	}
	// A third time there is nothing left to fill.
	if again, _ := s.ImportFile(ctx, strings.NewReader(endnoteXML), ImportOptions{NotOwned: "library"}); again.Filled != 0 || again.Already != 4 {
		t.Errorf("third time %+v", again)
	}
}
