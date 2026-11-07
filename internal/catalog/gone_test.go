package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestCopyGone(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get(ctx, id)
	copyID := b.Copies[0].ID

	gone, err := s.MarkGone(ctx, copyID, GoneInput{As: "donated", On: "2026-11-05", Note: "To the school library"})
	if err != nil || gone.GoneAs != "donated" || gone.GoneOn != "2026-11-05" {
		t.Fatalf("gone %+v, err %v", gone, err)
	}
	rows, _ := s.List(ctx)
	if rows[0].Copies != 0 {
		t.Errorf("a gone copy still counted: %d", rows[0].Copies)
	}
	queue, _ := s.LabelQueue(ctx)
	for _, l := range queue {
		if l.CopyID == copyID {
			t.Errorf("a gone copy's label still waits: %+v", l)
		}
	}
	b, _ = s.Get(ctx, id)
	if len(b.Copies) != 1 || len(b.ForVisitor().Copies) != 0 {
		t.Errorf("the owner keeps the record, a visitor does not see it: %d, %d", len(b.Copies), len(b.ForVisitor().Copies))
	}
	// Every copy gone, a visitor sees nothing of the book: not in the list, not by its page.
	every := &Scope{Books: "all", Wishlist: true}
	if every.ShowsBook(b) || len(every.Rows(rows)) != 0 {
		t.Errorf("a gone book shows to visitors: %v, %d rows", every.ShowsBook(b), len(every.Rows(rows)))
	}
	var conflict *ConflictError
	if _, err := s.Lend(ctx, copyID, LoanInput{Borrower: "Sam", LentOn: "2026-11-05"}); !errors.As(err, &conflict) {
		t.Errorf("a gone copy was lent: %v", err)
	}
	if _, err := s.MarkGone(ctx, copyID, GoneInput{As: "lost", On: "05/11/2026"}); err == nil {
		t.Error("a day not written YYYY-MM-DD was taken")
	}

	back, err := s.BringBack(ctx, copyID)
	if err != nil || back.GoneAs != "" {
		t.Fatalf("back %+v, err %v", back, err)
	}
	if rows, _ = s.List(ctx); rows[0].Copies != 1 {
		t.Errorf("a copy brought back is not counted: %d", rows[0].Copies)
	}

	// A copy out on loan comes back before it can go.
	if _, err := s.Lend(ctx, copyID, LoanInput{Borrower: "Sam", LentOn: "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkGone(ctx, copyID, GoneInput{As: "lost", On: "2026-11-06"}); !errors.As(err, &conflict) {
		t.Errorf("a copy out on loan was marked gone: %v", err)
	}

	if err := s.MarkSeen(ctx, copyID, "2026-11-07"); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetCopy(ctx, copyID); c.SeenOn != "2026-11-07" {
		t.Errorf("seen on %q", c.SeenOn)
	}
}

func TestReadAgain(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ReadStatus: "read", StartedOn: "2015-01-02", FinishedOn: "2015-03-02", Pages: 600}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReadAgain(ctx, id, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get(ctx, id)
	if b.ReadStatus != "reading" || b.StartedOn != "2026-11-05" || b.FinishedOn != "" ||
		len(b.Readings) != 1 || b.Readings[0].FinishedOn != "2015-03-02" {
		t.Fatalf("read again %+v", b)
	}
	var conflict *ConflictError
	if err := s.ReadAgain(ctx, id, "2026-11-06"); !errors.As(err, &conflict) {
		t.Errorf("a book being read was begun again: %v", err)
	}

	// Finished a second time, the year counts both reads.
	in := b.Input()
	in.ReadStatus, in.FinishedOn = "read", "2026-12-01"
	if err := s.Update(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	stats, err := s.Reading(ctx, nil)
	if err != nil || len(stats.Finished) != 2 || len(stats.Years) != 2 || stats.Years[0].Year != "2026" ||
		stats.Years[0].Pages != 600 || stats.Years[1].Year != "2015" {
		t.Fatalf("stats %+v, err %v", stats, err)
	}

	if err := s.DeleteReading(ctx, b.Readings[0].ID); err != nil {
		t.Fatal(err)
	}
	if b, _ = s.Get(ctx, id); len(b.Readings) != 0 {
		t.Errorf("an earlier read outlived its removal: %+v", b.Readings)
	}
}

func TestGaps(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	full := BookInput{Title: "Whole", ISBN: "9780425268285", ReadStatus: "unread", DDC: "823.92", HeightMM: 240, ThicknessMM: 30,
		Pages: 600, Format: "paperback", Publisher: "Ace", Published: "2014", Description: "A boy is given to the Sixth Order."}
	if _, err := s.Create(ctx, full, &CopyInput{PriceCents: -1}, ""); err != nil {
		t.Fatal(err)
	}
	thin, err := s.Create(ctx, BookInput{Title: "Thin", ISBN: "9780306406157", ReadStatus: "unread", Pages: 100}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, BookInput{Title: "Wanted", ReadStatus: "unread"}, nil, ""); err != nil {
		t.Fatal(err)
	}
	books, err := s.Gaps(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The whole book lacks only its cover; the wanted one is the wishlist's.
	if len(books) != 2 || books[0].Title != "Thin" || slices.Contains(books[0].Missing, "pages") || !slices.Contains(books[0].Missing, "ddc") {
		t.Errorf("books %+v", books)
	}
	if !slices.Equal(books[1].Missing, []string{"cover"}) {
		t.Errorf("the whole book lacks %v", books[1].Missing)
	}
	// Asked about one book, as the change feed names it, the list says nothing of the rest.
	if books, err = s.Gaps(ctx, []int64{thin, 9999}); err != nil || len(books) != 1 || books[0].BookID != thin {
		t.Errorf("only the thin book: %+v, err %v", books, err)
	}
	n, err := s.QueueLookups(ctx, []int64{thin, 9999})
	if err != nil || n != 1 {
		t.Errorf("queued %d, err %v", n, err)
	}
}

func TestReadStatus(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	mk := func(in BookInput, first *CopyInput) int64 {
		id, err := s.Create(ctx, in, first, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	one := mk(BookInput{Title: "Witcher 1", Series: "The Witcher", SeriesNumber: "1"}, &CopyInput{PriceCents: -1})
	two := mk(BookInput{Title: "Witcher 2", Series: "The Witcher", SeriesNumber: "2", ReadStatus: "read", FinishedOn: "2020-01-01"}, &CopyInput{PriceCents: -1})
	wanted := mk(BookInput{Title: "Witcher 3", Series: "The Witcher", SeriesNumber: "3"}, nil)

	// Begun today: that day is kept. Marked read: no day is made up for it.
	if err := s.SetReadStatus(ctx, one, ReadStatusInput{ReadStatus: "reading", On: "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetReadStatus(ctx, one, ReadStatusInput{ReadStatus: "read", On: "2026-11-09"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := s.Get(ctx, one); b.ReadStatus != "read" || b.StartedOn != "2026-11-05" || b.FinishedOn != "" {
		t.Errorf("one %s %s-%s", b.ReadStatus, b.StartedOn, b.FinishedOn)
	}
	var field *FieldError
	if err := s.SetReadStatus(ctx, one, ReadStatusInput{ReadStatus: "read", On: "soon"}); !errors.As(err, &field) {
		t.Errorf("a day that is not a day: %v", err)
	}

	// The series marked unread, then read: the wanted book is left alone, and no day made up.
	list, err := s.SeriesList(ctx, nil)
	if err != nil || len(list) != 1 {
		t.Fatalf("series %v, err %v", list, err)
	}
	n, err := s.SetSeriesRead(ctx, list[0].ID, SeriesReadInput{ReadStatus: "unread"})
	if err != nil || n != 2 {
		t.Fatalf("unread %d, err %v", n, err)
	}
	if n, err = s.SetSeriesRead(ctx, list[0].ID, SeriesReadInput{ReadStatus: "read"}); err != nil || n != 2 {
		t.Fatalf("read %d, err %v", n, err)
	}
	if b, _ := s.Get(ctx, two); b.ReadStatus != "read" || b.FinishedOn != "2020-01-01" {
		t.Errorf("two %s %s", b.ReadStatus, b.FinishedOn)
	}
	if b, _ := s.Get(ctx, wanted); b.ReadStatus != "unread" {
		t.Errorf("a wanted book was marked %s", b.ReadStatus)
	}
}
