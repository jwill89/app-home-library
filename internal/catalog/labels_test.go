package catalog

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

func TestCallNumber(t *testing.T) {
	for _, c := range []struct {
		name string
		in   db.CopyShelving
		want []string
	}{
		{"dewey", db.CopyShelving{Shelving: "dewey", Ddc: "153.42", AuthorSort: "Gladwell, Malcolm"}, []string{"153.42", "GLA"}},
		{"dewey without a number", db.CopyShelving{Shelving: "dewey", LabelPrefix: "REF", AuthorSort: "Webster, Noah"}, []string{"REF", "WEB"}},
		{"dewey with neither", db.CopyShelving{Shelving: "dewey", Title: "Untitled"}, []string{"UNT"}},
		{"fiction by author", db.CopyShelving{Shelving: "author", LabelPrefix: "FIC", Ddc: "823.92", AuthorSort: "McDonald, Ed", Series: "Raven's Mark", SeriesNumber: "3"}, []string{"FIC", "MCD", "v.3"}},
		{"manga by series", db.CopyShelving{Shelving: "series", LabelPrefix: "MANGA", AuthorSort: "Oda, Eiichiro", Series: "One Piece", SeriesNumber: "12"}, []string{"MANGA", "ONE", "v.12"}},
		{"series shelving without a series", db.CopyShelving{Shelving: "series", LabelPrefix: "MANGA", AuthorSort: "Oda, Eiichiro"}, []string{"MANGA", "ODA"}},
		{"title past its article", db.CopyShelving{Shelving: "author", LabelPrefix: "FIC", Title: "The Hobbit"}, []string{"FIC", "HOB"}},
		{"accents and apostrophes", db.CopyShelving{Shelving: "author", LabelPrefix: "FIC", AuthorSort: "Ó Faoláin, Seán"}, []string{"FIC", "OFA"}},
		{"a series article", db.CopyShelving{Shelving: "series", LabelPrefix: "GN", Series: "The Witcher", SeriesNumber: "1.5"}, []string{"GN", "WIT", "v.1.5"}},
	} {
		if got := CallNumber(&c.in); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestShelfOrder(t *testing.T) {
	fic := func(title, series, n string) db.CopyShelving {
		return db.CopyShelving{Section: "Fiction", Shelving: "author", LabelPrefix: "FIC", AuthorSort: "Abercrombie, Joe", Title: title, Series: series, SeriesNumber: n}
	}
	dewey := func(title, ddc, author string) db.CopyShelving {
		return db.CopyShelving{Section: "Nonfiction", Shelving: "dewey", Ddc: ddc, AuthorSort: author, Title: title}
	}
	in := []db.CopyShelving{
		dewey("Outliers", "302", "Gladwell, Malcolm"),
		fic("Half the World", "Shattered Sea", "2"),
		fic("Before They Are Hanged", "The First Law", "2"),
		dewey("Blink", "153.4", "Gladwell, Malcolm"),
		fic("Red Country", "", ""),
		fic("Half a King", "Shattered Sea", "1"),
		fic("The Blade Itself", "The First Law", "1"),
		fic("Last Argument of Kings", "The First Law", "10"),
	}
	sortShelved(in, func(c *db.CopyShelving) *db.CopyShelving { return c })
	var got []string
	for _, c := range in {
		got = append(got, c.Title)
	}
	// Standalones first, then each series together (The First Law sorts as First Law) in
	// volume order, v.2 before v.10; Nonfiction's Dewey numbers as numbers.
	want := []string{"Red Country", "The Blade Itself", "Before They Are Hanged", "Last Argument of Kings",
		"Half a King", "Half the World", "Blink", "Outliers"}
	if !slices.Equal(got, want) {
		t.Fatalf("shelf order %q, want %q", got, want)
	}
}

func TestLabelQueue(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, author, section, ddc string) *Book {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: title, DDC: ddc, Contributors: []lookup.Contributor{{Name: author, Role: "author"}}},
			&CopyInput{Section: section}, "")
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	crow := add("Crowfall", "Ed McDonald", "Fiction", "823.92")
	tipping := add("The Tipping Point", "Malcolm Gladwell", "Nonfiction", "302")
	abe := add("The Blade Itself", "Joe Abercrombie", "Fiction", "")
	// A section typed on a copy starts shelved by Dewey; Fiction is then set by author.
	if c := crow.Copies[0]; !slices.Equal(c.CallNumber, []string{"823.92", "MCD"}) {
		t.Fatalf("a new section's call number %q", c.CallNumber)
	}
	rules, err := s.ShelvingRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ruleOf := func(name string) ShelvingRule {
		for _, r := range rules {
			if r.Name == name {
				return r
			}
		}
		t.Fatalf("no section %s in %+v", name, rules)
		return ShelvingRule{}
	}
	if _, err := s.SetShelvingRule(ctx, ruleOf("Fiction").ID, ShelvingInput{Shelving: "author", Prefix: "FIC"}); err != nil {
		t.Fatal(err)
	}
	crow, _ = s.Get(ctx, crow.ID)

	// Spine labels are off at the start: they are queued, but the book says only what will print.
	if c := crow.Copies[0]; !slices.Equal(c.CallNumber, []string{"FIC", "MCD"}) || !slices.Equal(c.Labels, []string{"barcode"}) {
		t.Fatalf("a new copy's call number %q and labels %q", c.CallNumber, c.Labels)
	}
	if v := crow.ForVisitor().Copies[0]; len(v.CallNumber) != 2 || len(v.Labels) != 0 {
		t.Fatalf("a visitor sees the call number and not the queue: %+v", v)
	}

	queue, err := s.LabelQueue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var spines []string
	for _, l := range queue {
		if l.Kind == "spine" {
			spines = append(spines, l.Title)
		}
	}
	// By section, then by call number: Fiction's ABE before MCD, then Nonfiction.
	if !slices.Equal(spines, []string{"The Blade Itself", "Crowfall", "The Tipping Point"}) {
		t.Fatalf("spine labels in the order %q", spines)
	}

	pdf, err := s.LabelPDF(ctx, "spine", []int64{crow.Copies[0].ID, abe.Copies[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("(MCD) Tj")) || !bytes.Contains(pdf, []byte("(ABE) Tj")) || bytes.Contains(pdf, []byte("(GLA) Tj")) {
		t.Fatalf("the PDF holds the wrong labels")
	}
	if err := s.UnqueueLabels(ctx, "spine", []int64{crow.Copies[0].ID, abe.Copies[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LabelPDF(ctx, "spine", []int64{crow.Copies[0].ID}); err == nil {
		t.Fatal("a label already printed was drawn again")
	}
	if err := s.QueueLabel(ctx, "spine", crow.Copies[0].ID); err != nil {
		t.Fatal(err)
	}
	sheets, err := s.LabelSheets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sheets[0].Kind != "barcode" || sheets[0].Waiting != 3 || sheets[1].Waiting != 2 || sheets[1].PerSheet != 60 {
		t.Fatalf("sheets %+v", sheets)
	}
	// Spine labels start off; turned on, with an offset, they stay so.
	if !sheets[0].Enabled || sheets[1].Enabled {
		t.Fatalf("barcode labels on, spine labels off at the start: %+v", sheets)
	}
	if err := s.SetLabelSheet(ctx, "spine", LabelSheetInput{XMM: -2, YMM: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if sheets, _ = s.LabelSheets(ctx); sheets[1].XMM != -2 || sheets[1].YMM != 1 || !sheets[1].Enabled {
		t.Fatalf("settings not kept: %+v", sheets[1])
	}

	nonfiction := ruleOf("Nonfiction")
	if _, err := s.SetShelvingRule(ctx, nonfiction.ID, ShelvingInput{Shelving: "author"}); err == nil {
		t.Fatal("author shelving without a prefix was taken")
	}
	if _, err := s.SetShelvingRule(ctx, nonfiction.ID, ShelvingInput{Shelving: "author", Prefix: "non/fic"}); err == nil {
		t.Fatal("a prefix with a slash was taken")
	}
	r, err := s.SetShelvingRule(ctx, nonfiction.ID, ShelvingInput{Shelving: "author", Prefix: " nf "})
	if err != nil {
		t.Fatal(err)
	}
	if r.Prefix != "NF" || r.Copies != 1 {
		t.Fatalf("rule %+v", r)
	}
	if got, _ := s.Get(ctx, tipping.ID); !slices.Equal(got.Copies[0].CallNumber, []string{"NF", "GLA"}) {
		t.Fatalf("the call number did not follow the rule: %q", got.Copies[0].CallNumber)
	}
}

// The Bookshelf export's markup is cleaned by the background lookup, and a description it cut
// off stays known as cut once cleaned, so the page can say so.
func TestImportedDescriptionRepair(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	cut := "&&LI&&RBlood Song&&L/I&&R " + strings.Repeat("a", bookshelfCut-len("&&LI&&RBlood Song&&L/I&&R "))
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", Description: cut,
		Sources: map[string]string{"description": ImportSourceBookshelf}}, &CopyInput{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := s.Get(ctx, id); !b.DescriptionCut {
		t.Fatal("a 1500-character imported description is not known as cut")
	}
	if err := s.fill(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.Description, "&&L") || !strings.HasPrefix(b.Description, "Blood Song aaa") {
		t.Fatalf("the markup was not cleaned: %.60q", b.Description)
	}
	if !b.DescriptionCut || b.Sources["description"] != sourceBookshelfCut {
		t.Fatalf("no source had it whole, so it should stay marked cut: %v %q", b.DescriptionCut, b.Sources["description"])
	}
}
