package lookup

import "testing"

func TestParseHathiTrust(t *testing.T) {
	body := fixture(t, "hathitrust-9780226458083.json")
	r, err := ParseHathiTrust(body, "9780226458083")
	if err != nil || r == nil {
		t.Fatalf("record %v, err %v", r, err)
	}
	if r.Title == "" || r.LCC == "" || r.DDC == "" || r.HeightMM == 0 || len(r.Subjects) == 0 {
		t.Errorf("record %+v", r)
	}
	// A record that does not list the ISBN is not this book.
	if miss, _ := ParseHathiTrust(body, "9781111111113"); miss != nil {
		t.Errorf("a record without the ISBN was taken: %+v", miss)
	}
}

func TestMergeHathiTrust(t *testing.T) {
	// A cataloguer's class number comes before Open Library's; the book's own wording of its
	// title before a catalogue's.
	recs := map[string]*Record{
		SourceHathiTrust:  {Title: "The structure of scientific revolutions", DDC: "501", HeightMM: 210, Subjects: []string{"Science -- Philosophy"}},
		SourceOpenLibrary: {Title: "The Structure of Scientific Revolutions", DDC: "509"},
	}
	p := Merge("9780226458083", recs)
	if p.DDC != "501" || p.Sources["ddc"] != SourceHathiTrust || p.Title != "The Structure of Scientific Revolutions" || p.HeightMM != 210 {
		t.Errorf("proposal %+v", p)
	}
	if len(p.Subjects) != 1 {
		t.Errorf("subjects %v", p.Subjects)
	}
	// It names no series: what it gives as one is a publisher's line.
	recs[SourceHathiTrust].Series = "Bantam spectra book"
	if p = Merge("9780226458083", recs); p.Series != "" {
		t.Errorf("series %q", p.Series)
	}
	// With the Library of Congress' headings, HathiTrust's are not said again.
	recs[SourceHathiTrust] = &Record{Subjects: []string{"Science -- Philosophy"}}
	recs[SourceLoC] = &Record{DDC: "501", Subjects: []string{"Science -- Philosophy.", "Science -- History"}}
	if p = Merge("9780226458083", recs); p.Sources["ddc"] != SourceLoC || len(p.Subjects) != 2 {
		t.Errorf("with the LoC: %+v", p)
	}
}
