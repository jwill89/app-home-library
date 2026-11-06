package catalog

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"testing"
)

// A link shows its categories and its own books, the wanted ones only with the wishlist, and
// the series and the reading of what it shows; the owner, with no scope, sees everything.
func TestScopeBooks(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	mk := func(in BookInput, first *CopyInput) int64 {
		t.Helper()
		in.ReadStatus = cmp.Or(in.ReadStatus, "unread")
		id, err := s.Create(ctx, in, first, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	here := &CopyInput{PriceCents: -1}
	manga := mk(BookInput{Title: "Witch Hat Atelier 1", Category: "manga", Series: "Witch Hat Atelier", SeriesNumber: "1",
		ReadStatus: "read", FinishedOn: "2026-01-02"}, here)
	novel := mk(BookInput{Title: "Blood Song", Category: "fiction", Series: "Raven's Shadow", SeriesNumber: "1",
		ReadStatus: "read", FinishedOn: "2025-03-04"}, here)
	study := mk(BookInput{Title: "The Structure of Scientific Revolutions", Category: "nonfiction"}, here)
	wantedManga := mk(BookInput{Title: "Witch Hat Atelier 2", Category: "manga", Series: "Witch Hat Atelier", SeriesNumber: "2"}, nil)

	ids := func(rows []BookRow) []int64 {
		out := []int64{}
		for _, r := range rows {
			out = append(out, r.ID)
		}
		slices.Sort(out)
		return out
	}
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var owner *Scope
	if got := ids(owner.Rows(rows)); len(got) != 4 {
		t.Fatalf("the owner sees %v", got)
	}

	sc := &Scope{Books: "some", Categories: []string{"manga"}, BookIDs: []int64{study}, Papers: "none"}
	if err := sc.Normalize(); err != nil {
		t.Fatal(err)
	}
	if got, want := ids(sc.Rows(rows)), []int64{manga, study}; !slices.Equal(got, want) {
		t.Errorf("manga and one book: %v, want %v", got, want)
	}
	if b, _ := s.Get(ctx, novel); sc.ShowsBook(b) {
		t.Error("a novel shows by its page")
	}
	if b, _ := s.Get(ctx, wantedManga); sc.ShowsBook(b) {
		t.Error("a wanted book shows without the wishlist")
	}
	sc.Wishlist = true
	if got, want := ids(sc.Rows(rows)), []int64{manga, study, wantedManga}; !slices.Equal(got, want) {
		t.Errorf("with the wishlist: %v, want %v", got, want)
	}

	// The series and the reading name only what the link shows.
	series, err := s.SeriesList(ctx, sc)
	if err != nil || len(series) != 1 || series[0].Name != "Witch Hat Atelier" || series[0].Books != 1 ||
		!slices.Equal(series[0].Wanted, []string{"2"}) {
		t.Fatalf("series %+v, err %v", series, err)
	}
	if _, err := s.GetSeries(ctx, series[0].ID+1, sc); !errors.Is(err, ErrNotFound) {
		t.Errorf("a series the link does not show: %v", err)
	}
	reading, err := s.Reading(ctx, sc)
	if err != nil || len(reading.Finished) != 1 || reading.Finished[0].BookID != manga || len(reading.Years) != 1 {
		t.Fatalf("reading %+v, err %v", reading, err)
	}
	if all, _ := s.Reading(ctx, nil); len(all.Finished) != 2 {
		t.Errorf("the owner's reading %+v", all.Finished)
	}

	// A wanted book had in another edition names it only where the link shows that edition.
	for i := range rows {
		if rows[i].ID == wantedManga {
			rows[i].HadAs = novel
		}
	}
	for _, r := range sc.Rows(rows) {
		if r.HadAs != 0 {
			t.Errorf("%s names a book the link does not show: %d", r.Title, r.HadAs)
		}
	}

	none := &Scope{Books: "none", Wishlist: true, Papers: "all"}
	if err := none.Normalize(); err != nil || len(none.Rows(rows)) != 0 || none.Wishlist {
		t.Errorf("no books: %d rows, wishlist %v, err %v", len(none.Rows(rows)), none.Wishlist, err)
	}
	if got := none.Sees(); got.Books || got.Wishlist || !got.Papers {
		t.Errorf("no books sees %+v", got)
	}
	if got := owner.Sees(); !got.Books || !got.Wishlist || !got.Papers {
		t.Errorf("the owner sees %+v", got)
	}
}

func TestScopePapers(t *testing.T) {
	sc := &Scope{Books: "none", Papers: "some", CourseIDs: []int64{7, 7}, PaperIDs: []int64{3}}
	if err := sc.Normalize(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sc.CourseIDs, []int64{7}) {
		t.Errorf("courses kept twice: %v", sc.CourseIDs)
	}
	for _, c := range []struct {
		id, course int64
		want       bool
	}{{3, 0, true}, {4, 7, true}, {5, 8, false}, {6, 0, false}} {
		if got := sc.ShowsPaper(c.id, c.course); got != c.want {
			t.Errorf("paper %d of course %d: %v, want %v", c.id, c.course, got, c.want)
		}
	}
	if (&Scope{Papers: "none"}).ShowsPaper(3, 0) || !(&Scope{Papers: "all"}).ShowsPaper(3, 0) {
		t.Error("none or every paper")
	}
}

// A link chooses something, and keeps only the choices its kind of sharing uses.
func TestScopeNormalize(t *testing.T) {
	for _, c := range []struct {
		scope Scope
		field string
	}{
		{Scope{Books: "some", Papers: "none"}, "scope.books"},
		{Scope{Books: "all", Papers: "some"}, "scope.papers"},
		{Scope{Books: "none", Papers: "none"}, "scope"},
	} {
		var field *FieldError
		if err := c.scope.Normalize(); !errors.As(err, &field) || field.Field != c.field {
			t.Errorf("%+v: %v, want a problem with %s", c.scope, err, c.field)
		}
	}
	sc := Scope{Books: "all", Categories: []string{"manga"}, BookIDs: []int64{1}, Papers: "all", PaperIDs: []int64{2}}
	if err := sc.Normalize(); err != nil || len(sc.Categories) != 0 || len(sc.BookIDs) != 0 || len(sc.PaperIDs) != 0 {
		t.Errorf("choices kept beside every book and paper: %+v, %v", sc, err)
	}
}
