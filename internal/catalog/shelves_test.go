package catalog

import (
	"bytes"
	"context"
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

func plan(id int64, width, height int64, oversize bool) ShelfPlan {
	return ShelfPlan{Shelf: Shelf{ID: id, Position: id, ShelfInput: ShelfInput{WidthMM: width, HeightMM: height, DepthMM: 200, Oversize: oversize}}}
}

func book(id int64, height, thickness int64) ShelfBook {
	return ShelfBook{CopyID: id, HeightMM: height, ThicknessMM: thickness, key: &db.CopyShelving{CopyID: id, Title: string(rune('A' + id))}}
}

func TestPropose(t *testing.T) {
	// Two shelves 200 mm wide filled to half take two 40 mm books each; a tall book goes to the
	// oversize shelf and the next book still takes the shelf it was on.
	plans := []ShelfPlan{plan(1, 200, 250, false), plan(2, 200, 250, false), plan(3, 300, 400, true)}
	books := []ShelfBook{book(1, 200, 40), book(2, 200, 40), book(3, 320, 40), book(4, 200, 40), book(5, 200, 40), book(6, 200, 40), book(7, 450, 30)}
	unplaced, tall := propose(plans, books, 50)
	ids := func(bs []ShelfBook) []int64 {
		var out []int64
		for _, b := range bs {
			out = append(out, b.CopyID)
		}
		return out
	}
	if got := ids(plans[0].Books); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("first shelf %v", got)
	}
	if got := ids(plans[1].Books); len(got) != 2 || got[0] != 4 || got[1] != 5 {
		t.Fatalf("second shelf %v", got)
	}
	if got := ids(plans[2].Books); len(got) != 1 || got[0] != 3 {
		t.Fatalf("oversize shelf %v", got)
	}
	if got := ids(unplaced); len(got) != 1 || got[0] != 6 {
		t.Fatalf("no room for %v, want [6]", got)
	}
	if got := ids(tall); len(got) != 1 || got[0] != 7 {
		t.Fatalf("too tall for every shelf: %v, want [7]", got)
	}
}

func TestShelfPlanner(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, author, ddc string, height int64) int64 {
		t.Helper()
		id, err := s.Create(ctx, BookInput{Title: title, DDC: ddc, Pages: 300, HeightMM: height, ThicknessMM: 30,
			Contributors: []lookup.Contributor{{Name: author, Role: "author"}}}, &CopyInput{}, "")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := s.Get(ctx, id)
		return b.Copies[0].ID
	}
	blink := add("Blink", "Malcolm Gladwell", "153.4", 210)
	outliers := add("Outliers", "Malcolm Gladwell", "302", 210)
	atlas := add("Atlas of the World", "National Geographic", "912", 380)
	odyssey := add("The Odyssey", "Homer", "883", 210)

	if _, err := s.CreateBookcase(ctx, NewBookcase{Name: "  ", Shelves: 2, ShelfInput: ShelfInput{WidthMM: 100, HeightMM: 250, DepthMM: 250}}); err == nil {
		t.Fatal("a bookcase without a name was taken")
	}
	hall, err := s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: 2, ShelfInput: ShelfInput{WidthMM: 100, HeightMM: 250, DepthMM: 250}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddShelf(ctx, hall, ShelfInput{WidthMM: 400, HeightMM: 420, DepthMM: 300, Oversize: true}); err != nil {
		t.Fatal(err)
	}

	// Filled to 80%, a 100 mm shelf takes two 30 mm books.
	l, err := s.Layout(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if l.InUse || len(l.Shelves) != 3 || len(l.Shelves[0].Books) != 2 || len(l.Shelves[1].Books) != 1 || len(l.Shelves[2].Books) != 1 {
		t.Fatalf("proposal %+v", l)
	}
	if l.Shelves[0].Books[0].CopyID != blink || l.Shelves[0].Books[1].CopyID != outliers || l.Shelves[1].Books[0].CopyID != odyssey || l.Shelves[2].Books[0].CopyID != atlas {
		t.Fatalf("not in call number order, or the atlas is not on the oversize shelf: %+v", l.Shelves)
	}
	if l.Shelves[1].First[0] != "883" || l.Estimated != 0 {
		t.Fatalf("shelf range %q, estimated %d", l.Shelves[1].First, l.Estimated)
	}

	// Put to use, a new book goes where it belongs and nothing moves along.
	if err := s.UseLayout(ctx); err != nil {
		t.Fatal(err)
	}
	persuasion := add("Influence", "Robert Cialdini", "153.8", 210)
	l, _ = s.Layout(ctx, false)
	if !l.InUse || len(l.Shelves[0].Books) != 3 || l.Shelves[0].Books[1].CopyID != persuasion || len(l.Shelves[1].Books) != 1 {
		t.Fatalf("the layout in use moved books along: %+v", l.Shelves)
	}
	if l.Shelves[0].UsedMM != 90 {
		t.Fatalf("used %d mm", l.Shelves[0].UsedMM)
	}
	place, err := s.PlaceOf(ctx, persuasion)
	if err != nil {
		t.Fatal(err)
	}
	if !place.Placed || !place.InUse || place.Bookcase != "Hall" || place.Position != 1 || place.After != "Blink" || place.Before != "Outliers" {
		t.Fatalf("place %+v", place)
	}
	// A proposal, asked for while a layout is in use, starts afresh.
	if p, _ := s.Layout(ctx, true); p.InUse || len(p.Shelves[0].Books) != 2 {
		t.Fatalf("a fresh proposal %+v", p.Shelves[0])
	}

	pdf, err := s.RangeLabelsPDF(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pdf, []byte("(Hall · shelf 1)")) && !bytes.Contains(pdf, []byte(`(Hall \267 shelf 1)`)) {
		t.Fatalf("no shelf label for Hall: %.300s", pdf)
	}
	if !bytes.Contains(pdf, []byte("(to 302 GLA)")) {
		t.Fatalf("no range on the label: %.400s", pdf)
	}

	// Bookcases reorder; shelves renumber when one goes.
	study, err := s.CreateBookcase(ctx, NewBookcase{Name: "Study", Shelves: 1, ShelfInput: ShelfInput{WidthMM: 800, HeightMM: 300, DepthMM: 250}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateBookcase(ctx, study, BookcaseInput{Name: "Study", Position: 1}); err != nil {
		t.Fatal(err)
	}
	cases, _ := s.Bookcases(ctx)
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
	if l, _ = s.Layout(ctx, false); l.InUse {
		t.Fatal("still in use after stopping")
	}
}

// Corner braces 40 mm low and 20 mm in from each side: a book taller than 210 mm on a 250 mm
// shelf stands only between them.
func TestBraces(t *testing.T) {
	braced := func() []ShelfPlan {
		p := plan(1, 200, 250, false)
		p.BraceHeightMM, p.BraceWidthMM = 40, 20
		return []ShelfPlan{p, plan(2, 200, 250, false)}
	}

	// A tall first book leaves the strip under the left brace empty.
	plans := braced()
	propose(plans, []ShelfBook{book(1, 240, 40), book(2, 200, 40)}, 100)
	if p := plans[0]; p.GapMM != 20 || p.UsedMM != 100 || len(p.Books) != 2 {
		t.Fatalf("tall first book: gap %d, used %d, %d books", p.GapMM, p.UsedMM, len(p.Books))
	}

	// A short first book stands under it.
	plans = braced()
	propose(plans, []ShelfBook{book(1, 200, 40), book(2, 240, 40)}, 100)
	if p := plans[0]; p.GapMM != 0 || p.UsedMM != 80 {
		t.Fatalf("short first book: gap %d, used %d", p.GapMM, p.UsedMM)
	}

	// At the far end a tall book may not reach under the right brace, but a short one may.
	plans = braced()
	propose(plans, []ShelfBook{book(1, 200, 150), book(2, 240, 40)}, 100)
	if len(plans[0].Books) != 1 || len(plans[1].Books) != 1 {
		t.Fatalf("the tall book should have gone on: %d, %d", len(plans[0].Books), len(plans[1].Books))
	}
	plans = braced()
	propose(plans, []ShelfBook{book(1, 200, 150), book(2, 200, 40)}, 100)
	if len(plans[0].Books) != 2 {
		t.Fatal("a short book should fit under the right brace")
	}

	// Held to a shelf, a tall last book under the right brace shows as over full.
	plans = braced()
	plans[0].put(book(1, 200, 150))
	plans[0].put(book(2, 240, 40))
	finish(plans)
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
