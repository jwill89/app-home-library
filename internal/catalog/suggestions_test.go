package catalog

import (
	"context"
	"slices"
	"testing"
)

// A lookup the owner asked for keeps what the sources proposed: what they give otherwise is
// offered until it is taken, typed, or declined, and a declined value comes back only when a
// source gives another.
func TestSuggestionsKeptAndDeclined(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	const isbn = "9780425268285"
	p := s.Lookup.ByISBN(ctx, isbn, false).Proposal
	if p.Title == "" || p.Publisher == "" || p.Pages == 0 {
		t.Fatalf("the fixtures lack a title, publisher, or pages: %+v", p)
	}
	add := func(isbn string, in BookInput) int64 {
		t.Helper()
		in.ISBN, in.ReadStatus = isbn, "unread"
		id, err := s.Create(ctx, in, &CopyInput{PriceCents: -1}, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	offers := func(id int64) *Suggestions {
		t.Helper()
		sg, err := s.Suggested(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return sg
	}
	has := func(sg *Suggestions, want ...string) {
		t.Helper()
		for _, f := range want {
			if !slices.Contains(sg.Fields, f) {
				t.Errorf("%s not offered: %v", f, sg.Fields)
			}
		}
	}
	lacks := func(sg *Suggestions, gone ...string) {
		t.Helper()
		for _, f := range gone {
			if slices.Contains(sg.Fields, f) {
				t.Errorf("%s offered: %v", f, sg.Fields)
			}
		}
	}

	id := add(isbn, BookInput{Title: p.Title + " (Raven's Shadow, #1)", Publisher: "Not " + p.Publisher, Pages: int64(p.Pages) + 50})
	if sg := offers(id); len(sg.Fields) != 0 {
		t.Fatalf("offered before any lookup the owner asked for: %v", sg.Fields)
	}

	sg, err := s.Refresh(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	has(sg, "title", "publisher", "pages")
	list, err := s.SuggestionList(ctx, nil)
	if err != nil || len(list) != 1 || list[0].BookID != id || !slices.Equal(list[0].Fields, sg.Fields) {
		t.Fatalf("list %+v, err %v", list, err)
	}

	// Declined, the title stays as it is and is put back in what is suggested.
	if err := s.DeclineSuggestions(ctx, id, []string{"title", "lcc"}); err != nil {
		t.Fatal(err)
	}
	sg = offers(id)
	lacks(sg, "title")
	has(sg, "publisher", "pages")
	if sg.Suggested.Title != sg.Book.Title || sg.Suggested.Sources["title"] != sg.Book.Sources["title"] || sg.Suggested.Publisher != p.Publisher {
		t.Errorf("suggested title %q (from %q), publisher %q", sg.Suggested.Title, sg.Suggested.Sources["title"], sg.Suggested.Publisher)
	}

	// Taken by hand, the pages are no longer a difference.
	in := sg.Book.Input()
	in.Pages = int64(p.Pages)
	if err := s.Update(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	sg = offers(id)
	lacks(sg, "title", "pages")
	has(sg, "publisher")

	// A source that gives another title later offers it again.
	later := p
	later.Title = p.Title + ": A Novel"
	if err := s.keepProposal(ctx, id, &later); err != nil {
		t.Fatal(err)
	}
	has(offers(id), "title")

	// The background lookup keeps a proposal only when the owner asked for it.
	asked, added := add("9780547928227", BookInput{Title: "Asked For"}), add("9780316769488", BookInput{Title: "Just Added"})
	for _, c := range []struct {
		id    int64
		fresh bool
	}{{asked, true}, {added, false}} {
		if err := s.fill(ctx, c.id, c.fresh); err != nil {
			t.Fatal(err)
		}
		kept, _, err := s.proposal(ctx, c.id)
		if err != nil || (kept != nil) != c.fresh {
			t.Errorf("fresh %v: proposal kept %v, err %v", c.fresh, kept != nil, err)
		}
	}
}
