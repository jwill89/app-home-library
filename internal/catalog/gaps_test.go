package catalog

import (
	"context"
	"testing"

	"homelibrary/internal/db"
)

// Every book with an ISBN is looked up again, each source asked afresh; a book added meanwhile
// is looked up first, not after the whole library.
func TestQueueEveryLookup(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(in BookInput) int64 {
		t.Helper()
		in.ReadStatus = "unread"
		id, err := s.Create(ctx, in, &CopyInput{PriceCents: -1}, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	looked := add(BookInput{Title: "Blood Song", ISBN: "9780425268285"})
	add(BookInput{Title: "No ISBN"})
	q := db.New(s.DB.Write)
	if err := q.FinishEnrichment(ctx, looked); err != nil {
		t.Fatal(err)
	}

	n, err := s.QueueEveryLookup(ctx)
	if err != nil || n != 1 {
		t.Fatalf("queued %d, err %v", n, err)
	}
	next, err := s.read().NextEnrichment(ctx, now())
	if err != nil || next.EditionID != looked || next.Fresh != 1 {
		t.Fatalf("next %+v, err %v", next, err)
	}

	added := add(BookInput{Title: "The Hobbit", ISBN: "9780547928227"})
	if next, err = s.read().NextEnrichment(ctx, now()); err != nil || next.EditionID != added || next.Fresh != 0 {
		t.Fatalf("a book added meanwhile waits: next %+v, err %v", next, err)
	}
}
