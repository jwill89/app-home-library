package catalog

import (
	"slices"
	"testing"

	"homelibrary/internal/lookup"
)

func TestDifferences(t *testing.T) {
	in := BookInput{
		Title:        "Blood Song (Raven's Shadow, #1)",
		Contributors: []lookup.Contributor{{Name: "Anthony Ryan", Role: "author"}, {Name: "Jane Doe", Role: "translator"}},
		Publisher:    "Ace  Books",
		PublishPlace: "Lanham, Md.",
		Published:    "2014-07-01",
		Subtitle:     "Book One - A Raven's Shadow Novel",
		Format:       "hardcover",
		Pages:        591,
		HeightMM:     235,
		ThicknessMM:  40,
		Description:  "A story.",
		Sources:      map[string]string{"title": "bookshelf", "pages": "owner"},
	}
	p := lookup.Proposal{
		Title:        "Blood Song",
		Contributors: []lookup.Contributor{{Name: "Anthony Ryan", Role: "author"}}, // names fewer
		Publisher:    "Ace Books",                                                  // only the spacing differs
		PublishPlace: "Lanham, Md",                                                 // and the punctuation
		Published:    "2014",                                                       // the same date, less exactly
		Subtitle:     "book one : a raven's shadow novel",                          // the case and punctuation
		Format:       "mass_market",
		Pages:        624,
		HeightMM:     240, // within a centimetre
		ThicknessMM:  25,
		DDC:          "823.92", // a blank, filled before, never a suggestion
		Sources:      map[string]string{"title": "openlibrary", "format": "openlibrary", "pages": "openlibrary", "thicknessMm": "loc"},
	}
	got := differences(&in, &p)
	want := []string{"title", "format", "pages", "thicknessMm"}
	if !slices.Equal(got, want) {
		t.Fatalf("fields %v, want %v", got, want)
	}
	if in.Title != "Blood Song" || in.Pages != 624 || in.ThicknessMM != 25 || in.HeightMM != 235 || in.Publisher != "Ace  Books" {
		t.Errorf("suggested %+v", in)
	}
	if in.Sources["title"] != "openlibrary" || in.Sources["thicknessMm"] != "loc" || in.DDC != "" {
		t.Errorf("sources %v, ddc %q", in.Sources, in.DDC)
	}
}

func TestDifferencesPeople(t *testing.T) {
	in := BookInput{Contributors: []lookup.Contributor{{Name: "Gradmann, C", Role: "author"}}, Sources: map[string]string{}}
	p := lookup.Proposal{Contributors: []lookup.Contributor{{Name: "Christoph Gradmann", Role: "author"}}, Sources: map[string]string{"contributors": "google"}}
	if got := differences(&in, &p); !slices.Equal(got, []string{"contributors"}) || in.Contributors[0].Name != "Christoph Gradmann" {
		t.Errorf("fields %v, people %v", got, in.Contributors)
	}
	// The same name with other spacing is the same person; in another role, it is a change.
	in = BookInput{Contributors: []lookup.Contributor{{Name: "W.T. Jones", Role: "author"}}, Sources: map[string]string{}}
	p = lookup.Proposal{Contributors: []lookup.Contributor{{Name: "W. T. Jones", Role: "author"}}, Sources: map[string]string{}}
	if got := differences(&in, &p); len(got) != 0 {
		t.Errorf("only the spacing differs, yet %v", got)
	}
	p.Contributors[0].Role = "editor"
	if got := differences(&in, &p); !slices.Equal(got, []string{"contributors"}) {
		t.Errorf("another role: %v", got)
	}
}

// A date the source has more exactly is offered; one that disagrees is too.
func TestDifferencesDates(t *testing.T) {
	for _, c := range []struct {
		has, gives string
		offered    bool
	}{{"2014-07-01", "2014", false}, {"2014", "2014-07-01", true}, {"2014-07-01", "2015", true}, {"2014-07", "2014-08-02", true}} {
		in := BookInput{Published: c.has, Sources: map[string]string{}}
		p := lookup.Proposal{Published: c.gives, Sources: map[string]string{}}
		if got := slices.Contains(differences(&in, &p), "published"); got != c.offered {
			t.Errorf("has %s, source %s: offered %v", c.has, c.gives, got)
		}
	}
}
