package catalog

import (
	"context"
	"strings"
	"testing"

	"homelibrary/internal/lookup"
)

func TestWorkKey(t *testing.T) {
	same := [][4]string{
		{"The Structure of Scientific Revolutions", "Thomas S. Kuhn", "Structure of Scientific Revolutions: 50th Anniversary Edition", "Thomas Kuhn"},
		{"Dialogues Concerning Natural Religion, the Posthumous Essays", "David Hume", "Dialogues Concerning Natural Religion", "David Hume"},
		{"The Fabric of the Cosmos", "B. Greene", "The fabric of the cosmos : space, time", "Brian Greene"},
		// The surname is the one a name is filed under, past a suffix or a heading's comma.
		{"Why We Can't Wait", "Martin Luther King, Jr.", "Why We Can't Wait", "King, Martin Luther"},
		{"A Treatise of Human Nature", "Hume, David", "Treatise of Human Nature", "David Hume"},
	}
	for _, c := range same {
		if a, b := workKey(c[0], c[1]), workKey(c[2], c[3]); a == "" || a != b {
			t.Errorf("%q and %q are one work: %q, %q", c[0], c[2], a, b)
		}
	}
	if workKey("Selections", "Gottfried Wilhelm Leibniz") == workKey("Selections", "Immanuel Kant") {
		t.Error("two authors' Selections are one work")
	}
	if workKey("Selections", "") != "" {
		t.Error("a title alone makes a key")
	}
	// One year rule for every field a year is read from.
	for in, want := range map[string]string{"c1998.": "1998", "[1985]": "1985", "Ace (2014), 608 pages": "2014", "9781998000000": "", "": ""} {
		if got := lookup.Year(in); got != want {
			t.Errorf("Year(%q) = %q, want %q", in, got, want)
		}
	}
}

// A record EndNote has as one edition, of a book the house has in another.
const otherEditionXML = `<?xml version="1.0" encoding="UTF-8" ?><xml><records>` +
	`<record><rec-number>7</rec-number><foreign-keys><key app="EN" db-id="x">7</key></foreign-keys><ref-type name="Book">6</ref-type>` +
	`<contributors><authors><author><style face="normal" font="default" size="100%">Barrow, John D.</style></author></authors></contributors>` +
	`<titles><title><style face="normal" font="default" size="100%">The infinite book : a short guide to the boundless</style></title></titles>` +
	`<isbn><style face="normal" font="default" size="100%">9780375422270</style></isbn></record>` +
	`<record><rec-number>8</rec-number><foreign-keys><key app="EN" db-id="x">8</key></foreign-keys><ref-type name="Book">6</ref-type>` +
	`<contributors><authors><author><style face="normal" font="default" size="100%">Leibniz, Gottfried Wilhelm</style></author></authors></contributors>` +
	`<titles><title><style face="normal" font="default" size="100%">Selections</style></title></titles>` +
	`<isbn><style face="normal" font="default" size="100%">9780684146805</style></isbn></record>` +
	`</records></xml>`

func TestImportOtherEdition(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, author, isbn string) int64 {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: title, ISBN: isbn, ReadStatus: "unread",
			Contributors: []lookup.Contributor{{Name: author, Role: "author"}}}, &CopyInput{PriceCents: -1}, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	paperback := add("The Infinite Book", "John D. Barrow", "9781400032242")
	add("Selections", "Immanuel Kant", "9780023078217")

	rep, err := s.ImportFile(ctx, strings.NewReader(otherEditionXML), ImportOptions{NotOwned: "wishlist"})
	if err != nil {
		t.Fatal(err)
	}
	// The other edition of the owner's paperback is theirs; another author's Selections is wanted.
	if rep.Matched != 1 || rep.Wanted != 1 || rep.Added != 0 {
		t.Fatalf("report %+v", rep)
	}
	rows, _ := s.List(ctx)
	wanted := 0
	for _, r := range rows {
		if r.WantedOn == "" {
			continue
		}
		wanted++
		if r.Title != "Selections" || r.HadAs != 0 {
			t.Errorf("wanted %+v", r)
		}
	}
	if wanted != 1 {
		t.Errorf("%d wanted", wanted)
	}
	// Imported again, the matched record is known.
	if again, _ := s.ImportFile(ctx, strings.NewReader(otherEditionXML), ImportOptions{NotOwned: "wishlist"}); again.Already != 1 {
		t.Errorf("again %+v", again)
	}

	// A wanted book the house has in another edition says which it has.
	id, err := s.Create(ctx, BookInput{Title: "The Infinite Book", ISBN: "9780375422270", ReadStatus: "unread",
		Contributors: []lookup.Contributor{{Name: "John Barrow", Role: "author"}}}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	rows, _ = s.List(ctx)
	for _, r := range rows {
		if r.ID == id && r.HadAs != paperback {
			t.Errorf("had as %d, want %d", r.HadAs, paperback)
		}
	}
}
