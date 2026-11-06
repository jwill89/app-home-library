package catalog

import (
	"context"
	"errors"
	"testing"
)

func addBook(t *testing.T, s *Service, title string) *Book {
	t.Helper()
	id, err := s.Create(context.Background(), BookInput{Title: title}, &CopyInput{Section: "Fiction"}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func wantConflict(t *testing.T, err error) {
	t.Helper()
	var c *ConflictError
	if !errors.As(err, &c) {
		t.Fatalf("got %v, want a conflict", err)
	}
}

func wantField(t *testing.T, err error, field string) {
	t.Helper()
	var f *FieldError
	if !errors.As(err, &f) || f.Field != field {
		t.Fatalf("got %v, want a problem with %s", err, field)
	}
}

func TestLoans(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	b := addBook(t, s, "The Hobbit")
	copyID := b.Copies[0].ID

	_, err := s.Lend(ctx, copyID, LoanInput{Borrower: "Ana", LentOn: "2026-10-01", DueOn: "2026-09-01"})
	wantField(t, err, "dueOn")
	_, err = s.Lend(ctx, copyID, LoanInput{Borrower: "  ", LentOn: "2026-10-01"})
	wantField(t, err, "borrower")
	_, err = s.Lend(ctx, 999, LoanInput{Borrower: "Ana", LentOn: "2026-10-01"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("lending a copy that is not there: %v", err)
	}

	first, err := s.Lend(ctx, copyID, LoanInput{Borrower: " Ana ", LentOn: "2026-10-01", DueOn: "2026-10-22"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Borrower != "Ana" || first.Title != "The Hobbit" || first.Barcode != Barcode(copyID) || first.BookID != b.ID {
		t.Fatalf("loan %+v", first)
	}
	_, err = s.Lend(ctx, copyID, LoanInput{Borrower: "Ben", LentOn: "2026-10-02"})
	wantConflict(t, err)

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c := got.Copies[0]; !c.OnLoan || c.Loan == nil || c.Loan.Borrower != "Ana" {
		t.Fatalf("the book's copy does not show the loan: %+v", c)
	}
	if v := got.ForVisitor().Copies[0]; !v.OnLoan || v.Loan != nil {
		t.Fatalf("a visitor should see the copy out and no more: %+v", v)
	}

	wantConflict(t, s.DeleteCopy(ctx, copyID))
	wantConflict(t, s.Delete(ctx, b.ID))
	people, err := s.Borrowers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 1 || people[0].Out != 1 || people[0].Loans != 1 {
		t.Fatalf("borrowers %+v", people)
	}
	wantConflict(t, s.DeleteBorrower(ctx, people[0].ID))

	// Back, then out again to someone else; the first loan cannot be reopened over the second.
	back := first.LoanInput
	back.ReturnedOn = "2026-10-20"
	if _, err := s.UpdateLoan(ctx, first.ID, back); err != nil {
		t.Fatal(err)
	}
	second, err := s.Lend(ctx, copyID, LoanInput{Borrower: "ana", LentOn: "2026-10-25"})
	if err != nil {
		t.Fatal(err)
	}
	if second.BorrowerID != first.BorrowerID {
		t.Fatal("the same name in another case made a second borrower")
	}
	_, err = s.UpdateLoan(ctx, first.ID, first.LoanInput)
	wantConflict(t, err)

	loans, err := s.Loans(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loans) != 2 || loans[0].ID != second.ID || loans[1].ReturnedOn != "2026-10-20" {
		t.Fatalf("loans out first, then the history: %+v", loans)
	}

	ben, err := s.Lend(ctx, addBook(t, s, "Dune").Copies[0].ID, LoanInput{Borrower: "Ben", LentOn: "2026-10-03"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateBorrower(ctx, ben.BorrowerID, BorrowerInput{Name: "ANA"})
	wantConflict(t, err)
	renamed, err := s.UpdateBorrower(ctx, ben.BorrowerID, BorrowerInput{Name: "Ben Ito", Contact: "555-0100"})
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Ben Ito" || renamed.Out != 1 {
		t.Fatalf("renamed %+v", renamed)
	}

	// A borrower with nothing out goes, and their history with them.
	if err := s.DeleteLoan(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteBorrower(ctx, first.BorrowerID); err != nil {
		t.Fatal(err)
	}
	if loans, _ = s.Loans(ctx); len(loans) != 1 || loans[0].ID != ben.ID {
		t.Fatalf("after removing Ana: %+v", loans)
	}
	if err := s.DeleteCopy(ctx, copyID); err != nil {
		t.Fatalf("a copy back on the shelf could not be removed: %v", err)
	}
}

func TestCourses(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	b := addBook(t, s, "Calculus")
	copyID := b.Copies[0].ID

	_, err := s.CreateCourse(ctx, CourseInput{School: "State", Level: "unknown", Term: "unknown"})
	wantField(t, err, "title")
	_, err = s.CreateCourse(ctx, CourseInput{Number: "241", Level: "unknown", Term: "fall", Year: 99})
	wantField(t, err, "year")

	older, err := s.CreateCourse(ctx, CourseInput{Department: "MATH", Number: "241", Title: "Calculus III", Level: "undergraduate", Term: "spring", Year: 2004})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := s.CreateCourse(ctx, CourseInput{Title: " Number Theory ", Level: "unknown", Term: "fall", Year: 2004, Instructor: "Dr. Ruiz"})
	if err != nil {
		t.Fatal(err)
	}
	if newer.Title != "Number Theory" || newer.Level != "unknown" || len(newer.Books) != 0 {
		t.Fatalf("course %+v", newer)
	}
	list, err := s.Courses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != newer.ID {
		t.Fatalf("fall comes after spring in the same year, so first: %+v", list)
	}

	if _, err := s.UseInCourse(ctx, older.ID, copyID, false); err != nil {
		t.Fatal(err)
	}
	detail, err := s.UseInCourse(ctx, older.ID, copyID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Books) != 1 || !detail.Books[0].Required || detail.Books[0].BookID != b.ID || detail.BookCount != 1 {
		t.Fatalf("course books %+v", detail)
	}
	if _, err := s.UseInCourse(ctx, older.ID, 999, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a copy that is not there: %v", err)
	}

	got, err := s.Get(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Courses) != 1 || got.Courses[0].Course.Number != "241" || !got.Courses[0].Required {
		t.Fatalf("the book's courses %+v", got.Courses)
	}
	if len(got.ForVisitor().Courses) != 0 {
		t.Fatal("a visitor sees the courses")
	}

	if err := s.LeaveCourse(ctx, older.ID, copyID); err != nil {
		t.Fatal(err)
	}
	if err := s.LeaveCourse(ctx, older.ID, copyID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("leaving twice: %v", err)
	}
	if _, err := s.UseInCourse(ctx, newer.ID, copyID, false); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCourse(ctx, newer.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Get(ctx, b.ID); len(got.Courses) != 0 || len(got.Copies) != 1 {
		t.Fatalf("removing a course should keep its books and drop it from them: %+v", got)
	}
}
