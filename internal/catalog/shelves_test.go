package catalog

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

func TestBookSize(t *testing.T) {
	h, th, est := bookSize(&db.ListShelfCopiesRow{Format: "mass_market", Pages: 400})
	if h != 175 || th != 22 || !est {
		t.Fatalf("an unmeasured mass market of 400 pages: %d mm tall, %d mm thick, estimated %v", h, th, est)
	}
	if h, th, est := bookSize(&db.ListShelfCopiesRow{Format: "paperback", Pages: 300, HeightMm: 210, ThicknessMm: 25}); h != 210 || th != 25 || est {
		t.Fatalf("a measured book kept its measurements: %d %d %v", h, th, est)
	}
}

func plan(id, width, height, area int64) ShelfPlan {
	return ShelfPlan{Shelf: Shelf{ID: id, Position: id, AreaID: area, ShelfInput: ShelfInput{WidthMM: width, HeightMM: height, DepthMM: 200}}}
}

func book(id, height, thickness int64) ShelfBook {
	title := string(rune('a' + id))
	return ShelfBook{CopyID: id, Title: title, HeightMM: height, ThicknessMM: thickness, facts: bookFacts{title: title},
		key: areaKey{textPart(title), numPart(float64(id))}}
}

func ids(bs []ShelfBook) []int64 {
	out := []int64{}
	for _, b := range bs {
		out = append(out, b.CopyID)
	}
	return out
}

func TestPropose(t *testing.T) {
	// The general area's two shelves, 200 mm wide filled to half, take two 40 mm books each; a
	// book too tall for the shelf its turn comes on is set aside, and the next still takes that
	// shelf. The tall books' area has a shelf of its own.
	plans := []ShelfPlan{plan(1, 200, 250, 1), plan(2, 200, 250, 1), plan(3, 300, 400, 2), plan(4, 300, 400, 0)}
	areas := []Area{{ID: 2}, {ID: 1}, {ID: 3}}
	byArea := map[int64][]ShelfBook{
		1: {book(1, 200, 40), book(2, 200, 40), book(3, 320, 40), book(4, 200, 40), book(5, 200, 40), book(6, 200, 40)},
		2: {book(7, 320, 40)},
		3: {book(8, 200, 40)}, // an area with no shelves
	}
	unplaced, tall := propose(plans, areas, byArea, 50)
	for i, want := range [][]int64{{1, 2}, {4, 5}, {7}, {}} {
		if got := ids(plans[i].Books); !slices.Equal(got, want) {
			t.Errorf("shelf %d: %v, want %v", i+1, got, want)
		}
	}
	if got := ids(unplaced); !slices.Equal(got, []int64{6, 8}) {
		t.Errorf("no room for %v, want [6 8]", got)
	}
	if got := ids(tall); !slices.Equal(got, []int64{3}) {
		t.Errorf("too tall for their place: %v, want [3]", got)
	}
}

// Corner braces 40 mm low and 20 mm in from each side: a book taller than 210 mm on a 250 mm
// shelf stands only between them.
func TestBraces(t *testing.T) {
	areas := []Area{{ID: 1}}
	braced := func() []ShelfPlan {
		p := plan(1, 200, 250, 1)
		p.BraceHeightMM, p.BraceWidthMM = 40, 20
		return []ShelfPlan{p, plan(2, 200, 250, 1)}
	}
	lay := func(plans []ShelfPlan, books ...ShelfBook) {
		propose(plans, areas, map[int64][]ShelfBook{1: books}, 100)
	}

	// A tall first book leaves the strip under the left brace empty.
	plans := braced()
	lay(plans, book(1, 240, 40), book(2, 200, 40))
	if p := plans[0]; p.GapMM != 20 || p.UsedMM != 100 || len(p.Books) != 2 {
		t.Fatalf("tall first book: gap %d, used %d, %d books", p.GapMM, p.UsedMM, len(p.Books))
	}

	// A short first book stands under it.
	plans = braced()
	lay(plans, book(1, 200, 40), book(2, 240, 40))
	if p := plans[0]; p.GapMM != 0 || p.UsedMM != 80 {
		t.Fatalf("short first book: gap %d, used %d", p.GapMM, p.UsedMM)
	}

	// At the far end a tall book may not reach under the right brace, but a short one may.
	plans = braced()
	lay(plans, book(1, 200, 150), book(2, 240, 40))
	if len(plans[0].Books) != 1 || len(plans[1].Books) != 1 {
		t.Fatalf("the tall book should have gone on: %d, %d", len(plans[0].Books), len(plans[1].Books))
	}
	plans = braced()
	lay(plans, book(1, 200, 150), book(2, 200, 40))
	if len(plans[0].Books) != 2 {
		t.Fatal("a short book should fit under the right brace")
	}

	// Held to a shelf, a tall last book under the right brace shows as over full.
	plans = braced()
	plans[0].put(book(1, 200, 150))
	plans[0].put(book(2, 240, 40))
	finish(plans, areas)
	if plans[0].OverMM != 10 {
		t.Fatalf("over by %d mm, want 10", plans[0].OverMM)
	}
}

func TestBraceInput(t *testing.T) {
	in := ShelfInput{WidthMM: 800, HeightMM: 280, DepthMM: 280, BraceHeightMM: 38}
	if err := in.check(); err != nil || in.BraceHeightMM != 0 {
		t.Fatalf("a brace with no width is no brace: %v %+v", err, in)
	}
	in = ShelfInput{WidthMM: 800, HeightMM: 280, DepthMM: 280, BraceHeightMM: 300, BraceWidthMM: 19}
	if err := in.check(); err == nil {
		t.Fatal("a brace taller than the shelf was taken")
	}
}

// Each rule an area has must hold, and a book goes to the first area that takes it.
func TestAreaRules(t *testing.T) {
	b := ShelfBook{CopyID: 1, HeightMM: 240, facts: bookFacts{category: "nonfiction", dewey: 599.9, hasDewey: true,
		subjects: []string{"world war, 1939-1945"}, authorIDs: []int64{7}, seriesID: 3}}
	for _, c := range []struct {
		name string
		area Area
		want bool
	}{
		{"no rules", Area{}, true},
		{"its category", Area{AreaInput: AreaInput{AreaRules: AreaRules{Categories: []string{"fiction", "nonfiction"}}}}, true},
		{"another category", Area{AreaInput: AreaInput{AreaRules: AreaRules{Categories: []string{"manga"}}}}, false},
		{"a range to a whole number takes its decimals", Area{AreaInput: AreaInput{AreaRules: AreaRules{DeweyFrom: "500", DeweyTo: "599"}}}, true},
		{"a range with decimals ends there", Area{AreaInput: AreaInput{AreaRules: AreaRules{DeweyTo: "599.5"}}}, false},
		{"below the range", Area{AreaInput: AreaInput{AreaRules: AreaRules{DeweyFrom: "600"}}}, false},
		{"subject words", Area{AreaInput: AreaInput{AreaRules: AreaRules{Subject: "World War"}}}, true},
		{"other words", Area{AreaInput: AreaInput{AreaRules: AreaRules{Subject: "Astronomy"}}}, false},
		{"its author", Area{authorIDs: []int64{9, 7}}, true},
		{"another author", Area{authorIDs: []int64{9}}, false},
		{"its series", Area{seriesIDs: []int64{3}}, true},
		{"another series", Area{seriesIDs: []int64{4}}, false},
		{"tall enough", Area{AreaInput: AreaInput{AreaRules: AreaRules{MinHeightMM: 240}}}, true},
		{"too short", Area{AreaInput: AreaInput{AreaRules: AreaRules{MinHeightMM: 241}}}, false},
		{"too tall", Area{AreaInput: AreaInput{AreaRules: AreaRules{MaxHeightMM: 239}}}, false},
	} {
		if got := c.area.takes(&b); got != c.want {
			t.Errorf("%s: takes %v, want %v", c.name, got, c.want)
		}
	}
	noDewey := b
	noDewey.facts.hasDewey = false
	if (&Area{AreaInput: AreaInput{AreaRules: AreaRules{DeweyFrom: "500"}}}).takes(&noDewey) {
		t.Error("a Dewey range took a book without a Dewey number")
	}

	tall := Area{ID: 1, AreaInput: AreaInput{Order: []string{"title"}, AreaRules: AreaRules{MinHeightMM: 300}}}
	rest := Area{ID: 2, AreaInput: AreaInput{Order: []string{"title"}}}
	short, big := book(1, 200, 30), book(2, 320, 30)
	byArea, none := sorted([]Area{tall, rest}, []ShelfBook{short, big})
	if !slices.Equal(ids(byArea[1]), []int64{2}) || !slices.Equal(ids(byArea[2]), []int64{1}) || len(none) != 0 {
		t.Errorf("tall %v, rest %v, none %v", ids(byArea[1]), ids(byArea[2]), ids(none))
	}
	if _, none = sorted([]Area{tall}, []ShelfBook{short}); !slices.Equal(ids(none), []int64{1}) {
		t.Errorf("a book no area takes: %v", ids(none))
	}
}

// An area orders by its keys in turn: numbers as numbers, a book without the detail after
// those with it, a leading article left out of a title.
func TestAreaOrder(t *testing.T) {
	mk := func(id int64, f bookFacts) ShelfBook { return ShelfBook{CopyID: id, facts: f} }
	books := []ShelfBook{
		mk(1, bookFacts{title: "hobbit", // as filed, past its article
			author: "tolkien, john", series: "middle-earth", volume: "10"}),
		mk(2, bookFacts{title: "dune", author: "herbert, frank", dewey: 813, hasDewey: true}),
		mk(3, bookFacts{title: "atlas", dewey: 912, hasDewey: true}),
		mk(4, bookFacts{title: "blink", author: "gladwell, malcolm", dewey: 153.4, hasDewey: true}),
		mk(5, bookFacts{title: "silmarillion", author: "tolkien, john", series: "middle-earth", volume: "2"}),
	}
	order := func(keys ...string) []int64 {
		bs := slices.Clone(books)
		for i := range bs {
			bs[i].key = keyFor(&bs[i], keys)
		}
		slices.SortStableFunc(bs, func(a, b ShelfBook) int { return a.key.compare(b.key) })
		return ids(bs)
	}
	for _, c := range []struct {
		keys []string
		want []int64
	}{
		{[]string{"subject"}, []int64{4, 2, 3, 1, 5}},         // 153.4, 813, 912, then none, by title
		{[]string{"author"}, []int64{4, 2, 1, 5, 3}},          // no author last
		{[]string{"series", "title"}, []int64{5, 1, 3, 4, 2}}, // volume 2 before 10, then no series by title
		{[]string{"title"}, []int64{3, 4, 2, 1, 5}},
	} {
		if got := order(c.keys...); !slices.Equal(got, c.want) {
			t.Errorf("by %v: %v, want %v", c.keys, got, c.want)
		}
	}
}

// An area's rules are checked as they are saved, and its shelves leave any other area.
func TestAreaInput(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.Create(ctx, BookInput{Title: "Blink", ReadStatus: "unread", Series: "Pop Science",
		Contributors: []lookup.Contributor{{Name: "Malcolm Gladwell", Role: "author"}}}, &CopyInput{}, ""); err != nil {
		t.Fatal(err)
	}
	hall, err := s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: []ShelfInput{{WidthMM: 800, HeightMM: 300, DepthMM: 250}, {WidthMM: 800, HeightMM: 300, DepthMM: 250}}})
	if err != nil {
		t.Fatal(err)
	}
	cases, _ := s.Bookcases(ctx)
	top, bottom := cases[0].Shelves[0].ID, cases[0].Shelves[1].ID
	_ = hall

	field := func(err error) string {
		var f *FieldError
		if errors.As(err, &f) {
			return f.Field
		}
		return ""
	}
	for _, c := range []struct {
		in   AreaInput
		want string
	}{
		{AreaInput{Name: "  "}, "name"},
		{AreaInput{Name: "A", AreaRules: AreaRules{DeweyFrom: "5"}}, "deweyFrom"},
		{AreaInput{Name: "A", AreaRules: AreaRules{DeweyFrom: "600", DeweyTo: "500"}}, "deweyTo"},
		{AreaInput{Name: "A", AreaRules: AreaRules{MinHeightMM: 300, MaxHeightMM: 200}}, "maxHeightMm"},
		{AreaInput{Name: "A", Order: []string{"title", "title"}}, "order"},
		{AreaInput{Name: "A", AreaRules: AreaRules{Authors: []string{"Nobody Here"}}}, "authors"},
		{AreaInput{Name: "A", AreaRules: AreaRules{Series: []string{"No Such Series"}}}, "series"},
		{AreaInput{Name: "A", ShelfIDs: []int64{9999}}, "shelfIds"},
	} {
		if _, err := s.CreateArea(ctx, c.in); field(err) != c.want {
			t.Errorf("%+v: %v, want a problem with %s", c.in, err, c.want)
		}
	}

	nonfiction, err := s.CreateArea(ctx, AreaInput{Name: "Nonfiction", ShelfIDs: []int64{top, bottom},
		AreaRules: AreaRules{Authors: []string{"malcolm gladwell"}, Series: []string{"the pop science"}, DeweyFrom: "100", DeweyTo: "599"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateArea(ctx, AreaInput{Name: "NONFICTION"}); field(err) != "name" {
		t.Errorf("a second area of the same name: %v", err)
	}
	tall, err := s.CreateArea(ctx, AreaInput{Name: "Tall books", Position: 1, ShelfIDs: []int64{bottom}, AreaRules: AreaRules{MinHeightMM: 300}})
	if err != nil {
		t.Fatal(err)
	}
	areas, _ := s.Areas(ctx)
	if len(areas) != 2 || areas[0].ID != tall || areas[1].ID != nonfiction {
		t.Fatalf("order %+v", areas)
	}
	nf := areas[1]
	if !slices.Equal(nf.ShelfIDs, []int64{top}) || !slices.Equal(nf.Authors, []string{"Malcolm Gladwell"}) ||
		!slices.Equal(nf.Series, []string{"Pop Science"}) || !slices.Equal(nf.Order, defaultOrder) || nf.DeweyTo != "599" {
		t.Errorf("the shelf it gave up, the names as the library writes them, the order given: %+v", nf)
	}
	if err := s.DeleteArea(ctx, tall); err != nil {
		t.Fatal(err)
	}
	cases, _ = s.Bookcases(ctx)
	if cases[0].Shelves[1].AreaID != 0 || cases[0].Shelves[0].Area != "Nonfiction" {
		t.Errorf("shelves after the tall books' area went: %+v", cases[0].Shelves)
	}
}

func TestShelfPlanner(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, author, ddc, category string, height int64) int64 {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: title, DDC: ddc, Category: category, Pages: 300, HeightMM: height, ThicknessMM: 30,
			Contributors: []lookup.Contributor{{Name: author, Role: "author"}}}, &CopyInput{}, "")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := s.Get(ctx, id)
		return b.Copies[0].ID
	}
	blink := add("Blink", "Malcolm Gladwell", "153.4", "nonfiction", 210)
	outliers := add("Outliers", "Malcolm Gladwell", "302", "nonfiction", 210)
	atlas := add("Atlas of the World", "National Geographic", "912", "nonfiction", 380)
	odyssey := add("The Odyssey", "Homer", "883", "fiction", 210)

	if _, err := s.CreateBookcase(ctx, NewBookcase{Name: "  ", Shelves: []ShelfInput{{WidthMM: 100, HeightMM: 250, DepthMM: 250}}}); err == nil {
		t.Fatal("a bookcase without a name was taken")
	}
	hall, err := s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: []ShelfInput{{WidthMM: 100, HeightMM: 250, DepthMM: 250}, {WidthMM: 100, HeightMM: 250, DepthMM: 250}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddShelf(ctx, hall, ShelfInput{WidthMM: 400, HeightMM: 420, DepthMM: 300}); err != nil {
		t.Fatal(err)
	}
	cases, _ := s.Bookcases(ctx)
	shelf := func(i int) int64 { return cases[0].Shelves[i].ID }

	// No book has a place until a suggestion is asked for, and with no areas none is taken.
	if l, err := s.Layout(ctx, false); err != nil || l.InUse || l.Suggested || len(l.Shelves) != 3 || len(l.Shelves[0].Books) != 0 {
		t.Fatalf("unasked %+v, err %v", l, err)
	}
	if l, _ := s.Layout(ctx, true); len(l.NoArea) != 4 {
		t.Fatalf("with no areas, %d books in none", len(l.NoArea))
	}
	if p, _ := s.PlaceOf(ctx, blink); p.InUse || p.Placed || !p.Shelves {
		t.Fatalf("a place with no layout in use: %+v", p)
	}

	if _, err := s.CreateArea(ctx, AreaInput{Name: "Tall books", ShelfIDs: []int64{shelf(2)}, AreaRules: AreaRules{MinHeightMM: 300}}); err != nil {
		t.Fatal(err)
	}
	nonfiction, err := s.CreateArea(ctx, AreaInput{Name: "Nonfiction", ShelfIDs: []int64{shelf(0), shelf(1)},
		Order: []string{"subject", "author"}, AreaRules: AreaRules{Categories: []string{"nonfiction"}}})
	if err != nil {
		t.Fatal(err)
	}

	// Filled to 80%, a 100 mm shelf takes two 30 mm books; the atlas goes to the tall books'
	// area, tried first, and the Odyssey to none.
	l, err := s.Layout(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Suggested || l.InUse || !slices.Equal(ids(l.Shelves[0].Books), []int64{blink, outliers}) || len(l.Shelves[1].Books) != 0 ||
		!slices.Equal(ids(l.Shelves[2].Books), []int64{atlas}) || !slices.Equal(ids(l.NoArea), []int64{odyssey}) {
		t.Fatalf("suggestion %+v", l)
	}
	if l.Shelves[0].First != "153.4" || l.Shelves[0].Last != "302" || l.Shelves[0].Area != "Nonfiction" || l.Estimated != 0 {
		t.Fatalf("shelf range %q to %q in %q, estimated %d", l.Shelves[0].First, l.Shelves[0].Last, l.Shelves[0].Area, l.Estimated)
	}

	// Put to use, a new book goes where it belongs and nothing moves along.
	if err := s.UseLayout(ctx); err != nil {
		t.Fatal(err)
	}
	influence := add("Influence", "Robert Cialdini", "153.8", "nonfiction", 210)
	l, _ = s.Layout(ctx, false)
	if !l.InUse || l.Suggested || !slices.Equal(ids(l.Shelves[0].Books), []int64{blink, influence, outliers}) || l.Shelves[0].UsedMM != 90 {
		t.Fatalf("the layout in use moved books along: %+v", l.Shelves[0])
	}
	place, err := s.PlaceOf(ctx, influence)
	if err != nil {
		t.Fatal(err)
	}
	if !place.Placed || !place.InUse || place.Bookcase != "Hall" || place.Position != 1 || place.Area != "Nonfiction" ||
		place.After != "Blink" || place.Before != "Outliers" {
		t.Fatalf("place %+v", place)
	}
	// A suggestion, asked for while a layout is in use, starts afresh.
	if p, _ := s.Layout(ctx, true); !p.Suggested || len(p.Shelves[0].Books) != 2 {
		t.Fatalf("a fresh suggestion %+v", p.Shelves[0])
	}

	pdf, err := s.RangeLabelsPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("Hall")) || !bytes.Contains(pdf, []byte("Nonfiction")) || !bytes.Contains(pdf, []byte("(to 302)")) {
		t.Fatalf("no range label for Hall's first shelf: %.600s", pdf)
	}

	// An area given another order has shelves no longer in the layout: its books wait for the
	// next suggestion.
	areas, _ := s.Areas(ctx)
	nf := areas[slices.IndexFunc(areas, func(a Area) bool { return a.ID == nonfiction })]
	nf.Order = []string{"author"}
	if err := s.UpdateArea(ctx, nonfiction, nf.AreaInput); err != nil {
		t.Fatal(err)
	}
	l, _ = s.Layout(ctx, false)
	if !l.InUse || l.Shelves[0].InUse || len(l.Shelves[0].Books) != 0 || len(l.Unplaced) != 3 || !l.Shelves[2].InUse {
		t.Fatalf("after the order changed: %+v", l)
	}

	// Bookcases reorder; shelves renumber when one goes.
	study, err := s.CreateBookcase(ctx, NewBookcase{Name: "Study", Shelves: []ShelfInput{{WidthMM: 800, HeightMM: 300, DepthMM: 250}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateBookcase(ctx, study, BookcaseInput{Name: "Study", Position: 1}); err != nil {
		t.Fatal(err)
	}
	cases, _ = s.Bookcases(ctx)
	if cases[0].ID != study || cases[1].ID != hall {
		t.Fatalf("order %+v", cases)
	}
	if err := s.DeleteShelf(ctx, cases[1].Shelves[0].ID); err != nil {
		t.Fatal(err)
	}
	cases, _ = s.Bookcases(ctx)
	if len(cases[1].Shelves) != 2 || cases[1].Shelves[0].Position != 1 || cases[1].Shelves[1].Position != 2 {
		t.Fatalf("shelves after one went: %+v", cases[1].Shelves)
	}
	if err := s.StopLayout(ctx); err != nil {
		t.Fatal(err)
	}
	if l, _ = s.Layout(ctx, false); l.InUse || len(l.Shelves[1].Books) != 0 {
		t.Fatal("still in use after stopping")
	}
	if _, err := s.RangeLabelsPDF(ctx); err == nil {
		t.Error("shelf labels with no layout in use")
	}
}

// A new bookcase's shelves keep their own heights from the top down, and a shelf measured
// wrong is named by its place.
func TestBookcaseShelvesApart(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	shelf := func(h int64) ShelfInput {
		return ShelfInput{WidthMM: 800, HeightMM: h, DepthMM: 300, BraceHeightMM: 20, BraceWidthMM: 40}
	}
	if _, err := s.CreateBookcase(ctx, NewBookcase{Name: "Living room", Shelves: []ShelfInput{shelf(280), shelf(280), shelf(380)}}); err != nil {
		t.Fatal(err)
	}
	cases, err := s.Bookcases(ctx)
	if err != nil || len(cases) != 1 || len(cases[0].Shelves) != 3 {
		t.Fatalf("bookcases %+v, err %v", cases, err)
	}
	for i, want := range []int64{280, 280, 380} {
		if got := cases[0].Shelves[i]; got.Position != int64(i+1) || got.HeightMM != want || got.BraceHeightMM != 20 {
			t.Errorf("shelf %d: %+v, want height %d", i+1, got, want)
		}
	}

	_, err = s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: []ShelfInput{shelf(280), shelf(10)}})
	var field *FieldError
	if !errors.As(err, &field) || field.Field != "shelves[1].widthMm" || !strings.HasPrefix(field.Message, "Shelf 2: ") {
		t.Errorf("a shelf 1 cm tall: %v", err)
	}
	if cases, _ = s.Bookcases(ctx); len(cases) != 1 {
		t.Errorf("a bookcase with a shelf measured wrong was added: %d bookcases", len(cases))
	}
}
