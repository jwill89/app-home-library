package catalog

import (
	"context"
	"errors"
	"testing"
)

// saver is one form: what it reads now (the record's stamp) and a save of what it read, sent
// back with the stamp given.
type saver struct {
	what string
	read func(t *testing.T) (stamp string, save func(seen string) error)
}

// Every form's save goes through with the stamp its form read, or with none; is refused when
// the record has changed since; and moves the stamp each time, twice in one second included.
func TestSavesCheckTheirStamp(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	must := func(err error) { mustOK(t, err) }

	bookID, err := s.Create(ctx, BookInput{Title: "Blood Song", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	must(err)
	b, err := s.Get(ctx, bookID)
	must(err)
	copyID := b.Copies[0].ID
	loan, err := s.Lend(ctx, copyID, LoanInput{Borrower: "Ana", LentOn: "2026-11-01"})
	must(err)
	course, err := s.CreateCourse(ctx, CourseInput{Title: "Intro to Fantasy"})
	must(err)
	doc, err := s.CreateDocument(ctx, DocumentInput{Kind: "article", Title: "On Maps", Authors: []string{}, Editors: []string{}, Subjects: []string{}})
	must(err)
	areaID, err := s.CreateArea(ctx, AreaInput{Name: "Fiction", Order: []string{}, ShelfIDs: []int64{}})
	must(err)
	caseID, err := s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: []ShelfInput{{WidthMM: 800, HeightMM: 300, DepthMM: 250}}})
	must(err)

	bookcase := func(t *testing.T) Bookcase {
		t.Helper()
		cases, err := s.Bookcases(ctx)
		mustOK(t, err)
		for _, c := range cases {
			if c.ID == caseID {
				return c
			}
		}
		t.Fatal("the bookcase is gone")
		return Bookcase{}
	}

	forms := []saver{
		{"book", func(t *testing.T) (string, func(string) error) {
			b, err := s.Get(ctx, bookID)
			mustOK(t, err)
			in := b.Input()
			return b.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; return s.Update(ctx, bookID, in) }
		}},
		{"copy", func(t *testing.T) (string, func(string) error) {
			c, err := s.GetCopy(ctx, copyID)
			mustOK(t, err)
			in := c.CopyInput
			return c.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; _, err := s.UpdateCopy(ctx, copyID, in); return err }
		}},
		{"loan", func(t *testing.T) (string, func(string) error) {
			l, err := s.GetLoan(ctx, loan.ID)
			mustOK(t, err)
			in := l.LoanInput
			return l.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; _, err := s.UpdateLoan(ctx, loan.ID, in); return err }
		}},
		{"borrower", func(t *testing.T) (string, func(string) error) {
			all, err := s.Borrowers(ctx)
			mustOK(t, err)
			in := all[0].BorrowerInput
			return in.UpdatedAt, func(seen string) error {
				in.UpdatedAt = seen
				_, err := s.UpdateBorrower(ctx, all[0].ID, in)
				return err
			}
		}},
		{"course", func(t *testing.T) (string, func(string) error) {
			c, err := s.GetCourse(ctx, course.ID)
			mustOK(t, err)
			in := c.CourseInput
			return in.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; _, err := s.UpdateCourse(ctx, course.ID, in); return err }
		}},
		{"paper", func(t *testing.T) (string, func(string) error) {
			d, err := s.GetDocument(ctx, doc.ID)
			mustOK(t, err)
			in := d.DocumentInput
			return in.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; _, err := s.UpdateDocument(ctx, doc.ID, in); return err }
		}},
		{"area", func(t *testing.T) (string, func(string) error) {
			areas, err := s.Areas(ctx)
			mustOK(t, err)
			in := areas[0].AreaInput
			return in.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; return s.UpdateArea(ctx, areaID, in) }
		}},
		{"bookcase", func(t *testing.T) (string, func(string) error) {
			c := bookcase(t)
			in := BookcaseInput{Name: c.Name, Position: c.Position}
			return c.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; return s.UpdateBookcase(ctx, caseID, in) }
		}},
		{"shelf", func(t *testing.T) (string, func(string) error) {
			sh := bookcase(t).Shelves[0]
			in := sh.ShelfInput
			return in.UpdatedAt, func(seen string) error { in.UpdatedAt = seen; return s.UpdateShelf(ctx, sh.ID, in) }
		}},
	}
	for _, f := range forms {
		t.Run(f.what, func(t *testing.T) {
			first, save := f.read(t)
			if first == "" {
				t.Fatal("no stamp to read")
			}
			if err := save(first); err != nil {
				t.Fatalf("a save with the stamp it read: %v", err)
			}
			second, _ := f.read(t)
			if second <= first {
				t.Fatalf("the save left the stamp at %s, from %s", second, first)
			}
			var moved *MovedError
			if err := save(first); !errors.As(err, &moved) {
				t.Fatalf("a save from before the last change: %v", err)
			}
			if err := save(""); err != nil {
				t.Fatalf("a save that asks for no check: %v", err)
			}
			if third, _ := f.read(t); third <= second {
				t.Errorf("a second save in the same second left the stamp at %s, from %s", third, second)
			}
		})
	}
}

// A change the form shows but doesn't make itself moves the stamp too: a series renamed, for
// each of its books, and a shelf taken by another area, for the area that had it.
func TestOtherChangesMoveTheStamp(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	bookID, err := s.Create(ctx, BookInput{Title: "Blood Song", Series: "Raven's Shadow", SeriesNumber: "1", ReadStatus: "unread"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Get(ctx, bookID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.RenameSeries(ctx, before.SeriesID, SeriesInput{Name: "Raven's Shadow Trilogy"}); err != nil {
		t.Fatal(err)
	}
	after, err := s.Get(ctx, bookID)
	if err != nil {
		t.Fatal(err)
	}
	if after.UpdatedAt <= before.UpdatedAt {
		t.Errorf("a book's series renamed left its stamp at %s", after.UpdatedAt)
	}

	caseID, err := s.CreateBookcase(ctx, NewBookcase{Name: "Hall", Shelves: []ShelfInput{{WidthMM: 800, HeightMM: 300, DepthMM: 250}}})
	if err != nil {
		t.Fatal(err)
	}
	cases, err := s.Bookcases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var shelf int64
	for _, c := range cases {
		if c.ID == caseID {
			shelf = c.Shelves[0].ID
		}
	}
	first, err := s.CreateArea(ctx, AreaInput{Name: "Fiction", Order: []string{}, ShelfIDs: []int64{shelf}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := func(id int64) string {
		t.Helper()
		areas, err := s.Areas(ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range areas {
			if a.ID == id {
				return a.UpdatedAt
			}
		}
		t.Fatal("no such area")
		return ""
	}
	had := stamp(first)
	if _, err := s.CreateArea(ctx, AreaInput{Name: "Tall books", Order: []string{}, ShelfIDs: []int64{shelf}}); err != nil {
		t.Fatal(err)
	}
	if now := stamp(first); now <= had {
		t.Errorf("an area that lost its shelf kept its stamp at %s", now)
	}
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
