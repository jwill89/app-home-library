package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"

	"homelibrary/internal/lookup"
)

func TestLeaveSeries(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.fill(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get(ctx, id)
	if b.Series != "Raven's Shadow" || b.SeriesNumber != "1" || b.SeriesID == 0 {
		t.Fatalf("filled series %q #%q (%d)", b.Series, b.SeriesNumber, b.SeriesID)
	}
	if err := s.LeaveSeries(ctx, b.SeriesID+1, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("left a series it was not in: %v", err)
	}
	if err := s.LeaveSeries(ctx, b.SeriesID, id); err != nil {
		t.Fatal(err)
	}
	// The owner took it out: a lookup does not put it back.
	if err := s.fill(ctx, id, false); err != nil {
		t.Fatal(err)
	}
	if b, _ = s.Get(ctx, id); b.Series != "" || b.SeriesNumber != "" || b.Sources["series"] != SourceOwner {
		t.Errorf("after leaving: %q #%q, sources %v", b.Series, b.SeriesNumber, b.Sources)
	}
}

func TestDismissSeries(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, series, source string) int64 {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: title, Series: series, ReadStatus: "unread", Sources: map[string]string{"series": source}}, &CopyInput{PriceCents: -1}, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := add("Meditations", "Penguin classics", "openlibrary")
	add("The Odyssey", "Penguin classics", "openlibrary")
	b, _ := s.Get(ctx, first)
	n, err := s.DismissSeries(ctx, b.SeriesID)
	if err != nil || n != 2 {
		t.Fatalf("dismissed %d, err %v", n, err)
	}
	if list, _ := s.SeriesList(ctx, nil); len(list) != 0 {
		t.Errorf("series left %+v", list)
	}
	if c, _ := s.Choices(ctx); slices.Contains(c.Series, "Penguin classics") {
		t.Errorf("the form still offers it: %v", c.Series)
	}
	// From a source, the name puts no book in it; typed by the owner, it is a series again.
	if b, _ := s.Get(ctx, add("The Iliad", "Penguin Classics", "openlibrary")); b.Series != "" {
		t.Errorf("a source put a book in a series put aside: %q", b.Series)
	}
	if b, _ := s.Get(ctx, add("The Aeneid", "penguin classics", SourceOwner)); b.Series != "Penguin classics" {
		t.Errorf("the owner's series %q", b.Series)
	}
	if list, _ := s.SeriesList(ctx, nil); len(list) != 1 {
		t.Errorf("taken back: %+v", list)
	}
}

func TestSeriesWrittenAnotherWay(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	for _, name := range []string{"The Broken Empire", "Broken Empire", "broken empire series"} {
		if _, err := s.Create(ctx, BookInput{Title: name, Series: name, ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, ""); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := s.SeriesList(ctx, nil)
	if len(list) != 1 || list[0].Name != "The Broken Empire" || list[0].Books != 3 {
		t.Errorf("series %+v", list)
	}
	for in, want := range map[string]string{"The Broken Empire": "broken empire", "Raven's Blade Novel, A": "raven s blade novel", "Dimensions of Philosophy Series": "dimensions of philosophy", "The": "the"} {
		if got := seriesKey(in); got != want {
			t.Errorf("seriesKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFillBlanksLeavesTheOwnersBlanks(t *testing.T) {
	in := BookInput{Sources: map[string]string{"publisher": SourceOwner, "format": SourceOwner}}
	p := lookup.Proposal{Publisher: "Ace", Format: "hardcover", Pages: 300, Sources: map[string]string{"publisher": "loc", "pages": "loc", "format": "loc"}}
	fillBlanks(&in, &p)
	if in.Publisher != "" || in.Format != "" || in.Pages != 300 {
		t.Errorf("filled %+v", in)
	}
	// Subjects the owner set stay as the owner left them: none added, none put back.
	mine := BookInput{Subjects: []string{"Mine"}, Sources: map[string]string{"subjects": SourceOwner}}
	fillBlanks(&mine, &lookup.Proposal{Subjects: []string{"Removed once", "Another"}, Sources: map[string]string{}})
	if len(mine.Subjects) != 1 {
		t.Errorf("the owner's subjects became %v", mine.Subjects)
	}
}

func TestDifferencesSeries(t *testing.T) {
	in := BookInput{Series: "Malazan book of the fallen -- bk. 9", Sources: map[string]string{}}
	p := lookup.Proposal{Series: "Malazan Book of the Fallen", SeriesNumber: "9", Sources: map[string]string{"series": "openlibrary", "seriesNumber": "openlibrary"}}
	if got := differences(&in, &p); !slices.Equal(got, []string{"series", "seriesNumber"}) || in.SeriesNumber != "9" {
		t.Errorf("fields %v, number %q", got, in.SeriesNumber)
	}
	same := BookInput{Series: "The Broken Empire", SeriesNumber: "2", Sources: map[string]string{}}
	if got := differences(&same, &lookup.Proposal{Series: "Broken Empire", SeriesNumber: "2", Sources: map[string]string{}}); len(got) != 0 {
		t.Errorf("the same series written another way: %v", got)
	}
}

func TestRenameFindsTheSeriesWrittenAnotherWay(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(series string) int64 {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: series + " book", Series: series, ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := s.Get(ctx, id)
		return b.SeriesID
	}
	theirs := add("The Broken Empire")
	stray := add("Prince of Thorns")
	keep, err := s.RenameSeries(ctx, stray, SeriesInput{Name: "broken empire series"})
	if err != nil || keep != theirs {
		t.Fatalf("kept %d, want %d: %v", keep, theirs, err)
	}
	if list, _ := s.SeriesList(ctx, nil); len(list) != 1 || list[0].Books != 2 {
		t.Errorf("series %+v", list)
	}
}

func TestInert(t *testing.T) {
	for in, want := range map[string]string{"=HYPERLINK(\"x\")": "'=HYPERLINK(\"x\")", "+1": "'+1", "@SUM": "'@SUM", "Plain": "Plain", "": ""} {
		if got := inert(in); got != want {
			t.Errorf("inert(%q) = %q, want %q", in, got, want)
		}
	}
}
