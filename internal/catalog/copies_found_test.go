package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOpenCopy(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, doi string) int64 {
		t.Helper()
		d, err := s.CreateDocument(ctx, DocumentInput{Title: title, Kind: "article", DOI: doi})
		if err != nil {
			t.Fatal(err)
		}
		return d.ID
	}
	open := add("An Open Paper", "10.1000/open")
	c, err := s.OpenCopy(ctx, open)
	if err != nil || c == nil || c.License != "cc-by" || c.Host != "A Repository" {
		t.Fatalf("copy %+v, err %v", c, err)
	}
	d, err := s.AddOpenCopy(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 || d.Files[0].Name != "An Open Paper.pdf" {
		t.Errorf("files %+v", d.Files)
	}

	// A link that gives a page is refused, saying what to do; no DOI, no copy.
	var conflict *ConflictError
	if _, err := s.AddOpenCopy(ctx, add("A Page", "10.1000/page")); !errors.As(err, &conflict) || !strings.Contains(conflict.Message, "web page") {
		t.Errorf("a page was taken: %v", err)
	}
	none := add("No DOI", "")
	if c, err := s.OpenCopy(ctx, none); c != nil || err != nil {
		t.Errorf("without a DOI: %+v, %v", c, err)
	}
	if _, err := s.AddOpenCopy(ctx, none); !errors.As(err, &conflict) {
		t.Errorf("without a DOI: %v", err)
	}
}

func TestFillFromCitation(t *testing.T) {
	s := newService(t)
	draft := DocumentInput{Kind: "other", Title: "My Own Title", Subjects: []string{"Philosophy"}}
	ris := "TY  - JOUR\nT1  - A Paper\nAU  - Doe, Jane\nJO  - A Journal\nKW  - philosophy\nKW  - Science\nER  - \n"
	f, err := s.FillFromCitation(ris, draft)
	if err != nil {
		t.Fatal(err)
	}
	// What the owner typed stays; the blanks take the record's, and the subjects gain its new one.
	d := f.Document
	if d.Title != "My Own Title" || d.Kind != "article" || d.Container != "A Journal" || len(d.Authors) != 1 ||
		d.Authors[0] != "Jane Doe" || strings.Join(d.Subjects, "; ") != "Philosophy; Science" {
		t.Errorf("filled %+v", d)
	}
	if got := strings.Join(f.Filled, ","); got != "kind,authors,container,subjects" {
		t.Errorf("fields %s", got)
	}
	// The same record again has nothing left to give.
	if again, _ := s.FillFromCitation(ris, d); len(again.Filled) != 0 {
		t.Errorf("filled again %v", again.Filled)
	}
	var field *FieldError
	if _, err := s.FillFromCitation("%PDF-1.4", draft); !errors.As(err, &field) {
		t.Errorf("a file without a citation: %v", err)
	}
}

// What a DOI gives otherwise is offered; spacing, a date the paper has more exactly, an article
// for a review, the same people written more fully, and Crossref's publisher are not.
func TestDocumentDifferences(t *testing.T) {
	have := DocumentInput{Kind: "review", Title: "The  Dappled World", Authors: []string{"N. Cartwright", "John D. Norton"},
		Container: "Mind", Published: "1999-11-02", Pages: "1-5", Publisher: "Routledge"}
	from := DocumentInput{Kind: "article", Title: "The Dappled World", Authors: []string{"Nancy Cartwright", "John D Norton"},
		Container: "Mind", Published: "1999", Pages: "1-6", Publisher: "Informa UK Limited"}
	suggested, fields := documentDifferences(have, from)
	if strings.Join(fields, ",") != "pages" || suggested.Pages != "1-6" || suggested.Kind != "review" ||
		suggested.Publisher != "Routledge" {
		t.Fatalf("fields %v, suggested %+v", fields, suggested)
	}
	// Other people are offered.
	if suggested, fields := documentDifferences(DocumentInput{Authors: []string{"Hacking"}}, DocumentInput{Authors: []string{"Ian Hacking", "Bas van Fraassen"}}); strings.Join(fields, ",") != "authors" || len(suggested.Authors) != 2 {
		t.Fatalf("authors %v %+v", fields, suggested)
	}
	// A chapter Crossref says is a chapter, where the paper says article, is offered.
	if _, fields := documentDifferences(DocumentInput{Kind: "article"}, DocumentInput{Kind: "chapter"}); strings.Join(fields, ",") != "kind" {
		t.Fatalf("kind %v", fields)
	}
}
