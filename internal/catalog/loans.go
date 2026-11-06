package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"homelibrary/internal/db"
)

// LoanInput is a loan as the owner records it. The borrower is named, and a name the library
// has not seen before is a new borrower.
type LoanInput struct {
	Borrower   string `json:"borrower" minLength:"1" maxLength:"100"`
	LentOn     string `json:"lentOn" doc:"YYYY-MM-DD"`
	DueOn      string `json:"dueOn" doc:"YYYY-MM-DD, or empty for no due date"`
	ReturnedOn string `json:"returnedOn" doc:"YYYY-MM-DD, or empty while the copy is out"`
	Notes      string `json:"notes" maxLength:"2000"`
}

// Loan is one copy lent once, with the book it is a copy of.
type Loan struct {
	ID         int64  `json:"id"`
	CopyID     int64  `json:"copyId"`
	Barcode    string `json:"barcode"`
	BookID     int64  `json:"bookId"`
	Title      string `json:"title"`
	Authors    string `json:"authors"`
	Cover      Cover  `json:"cover"`
	BorrowerID int64  `json:"borrowerId"`
	LoanInput
}

// BorrowerInput is what the owner keeps about someone they lend to.
type BorrowerInput struct {
	Name    string `json:"name" minLength:"1" maxLength:"100"`
	Contact string `json:"contact" maxLength:"200" doc:"A phone number or an address to reach them by"`
	Notes   string `json:"notes" maxLength:"2000"`
}

// Borrower is someone the owner lends to, with how many of the library's copies they have.
type Borrower struct {
	ID int64 `json:"id"`
	BorrowerInput
	Out   int64 `json:"out" doc:"Copies they have now"`
	Loans int64 `json:"loans" doc:"Every loan, returned or not"`
}

func (in *LoanInput) normalize() error {
	in.Borrower = strings.TrimSpace(in.Borrower)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Borrower == "" {
		return &FieldError{"borrower", "say who has the book"}
	}
	if !dayRe.MatchString(in.LentOn) {
		return &FieldError{"lentOn", "write the day it was lent as YYYY-MM-DD"}
	}
	for _, d := range []struct{ field, name, value string }{
		{"dueOn", "the due date", in.DueOn},
		{"returnedOn", "the return date", in.ReturnedOn},
	} {
		if d.value == "" {
			continue
		}
		if !dayRe.MatchString(d.value) {
			return &FieldError{d.field, "write " + d.name + " as YYYY-MM-DD"}
		}
		// ISO days compare as text.
		if d.value < in.LentOn {
			return &FieldError{d.field, d.name + " comes before the day it was lent; choose a later day"}
		}
	}
	return nil
}

func (in *BorrowerInput) normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	in.Contact = strings.TrimSpace(in.Contact)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Name == "" {
		return &FieldError{"name", "a borrower needs a name"}
	}
	return nil
}

// Lend records a copy going out.
func (s *Service) Lend(ctx context.Context, copyID int64, in LoanInput) (*Loan, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		if c, err := q.GetCopy(ctx, copyID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		} else if c.GoneAs != "" {
			return &ConflictError{fmt.Sprintf("%s has gone from the library. Bring it back first.", Barcode(copyID))}
		}
		if err := stillHere(ctx, q, copyID, 0, in); err != nil {
			return err
		}
		borrower, err := q.UpsertBorrower(ctx, in.Borrower)
		if err != nil {
			return fmt.Errorf("borrower: %w", err)
		}
		id, err = q.CreateLoan(ctx, db.CreateLoanParams{
			CopyID: copyID, BorrowerID: borrower, LentOn: in.LentOn, DueOn: in.DueOn,
			ReturnedOn: in.ReturnedOn, Notes: in.Notes,
		})
		if err != nil {
			return fmt.Errorf("create loan: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetLoan(ctx, id)
}

// UpdateLoan replaces a loan's details; a return date marks the copy back.
func (s *Service) UpdateLoan(ctx context.Context, id int64, in LoanInput) (*Loan, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		l, err := q.GetLoan(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := stillHere(ctx, q, l.CopyID, id, in); err != nil {
			return err
		}
		borrower, err := q.UpsertBorrower(ctx, in.Borrower)
		if err != nil {
			return fmt.Errorf("borrower: %w", err)
		}
		return q.UpdateLoan(ctx, db.UpdateLoanParams{
			BorrowerID: borrower, LentOn: in.LentOn, DueOn: in.DueOn, ReturnedOn: in.ReturnedOn,
			Notes: in.Notes, ID: id,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetLoan(ctx, id)
}

// stillHere refuses a loan that would put a copy with two borrowers at once.
func stillHere(ctx context.Context, q *db.Queries, copyID, except int64, in LoanInput) error {
	if in.ReturnedOn != "" {
		return nil
	}
	name, err := q.CopyOutTo(ctx, db.CopyOutToParams{CopyID: copyID, ExceptLoan: except})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ConflictError{fmt.Sprintf("%s is already lent to %s. Mark it returned first.", Barcode(copyID), name)}
}

// DeleteLoan removes a loan from the history, as for one recorded by mistake.
func (s *Service) DeleteLoan(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetLoan(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		return q.DeleteLoan(ctx, id)
	})
}

// GetLoan returns one loan.
func (s *Service) GetLoan(ctx context.Context, id int64) (*Loan, error) {
	l, err := s.read().GetLoan(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := s.loanOf(l)
	return &out, nil
}

// Loans lists every loan: those out first, then the history, latest first.
func (s *Service) Loans(ctx context.Context) ([]Loan, error) {
	rows, err := s.read().ListLoans(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Loan, 0, len(rows))
	for _, l := range rows {
		out = append(out, s.loanOf(l))
	}
	return out, nil
}

func (s *Service) loanOf(l *db.LoanRow) Loan {
	return Loan{
		ID: l.ID, CopyID: l.CopyID, Barcode: Barcode(l.CopyID), BookID: l.EditionID, Title: l.Title,
		Authors: l.Authors, Cover: s.cover(l.Cover), BorrowerID: l.BorrowerID,
		LoanInput: LoanInput{
			Borrower: l.Borrower, LentOn: l.LentOn, DueOn: l.DueOn, ReturnedOn: l.ReturnedOn, Notes: l.Notes,
		},
	}
}

// Borrowers lists everyone the owner has lent to.
func (s *Service) Borrowers(ctx context.Context) ([]Borrower, error) {
	rows, err := s.read().ListBorrowers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Borrower, 0, len(rows))
	for _, b := range rows {
		out = append(out, borrowerOf((*db.GetBorrowerRow)(b)))
	}
	return out, nil
}

// UpdateBorrower renames a borrower or changes how to reach them.
func (s *Service) UpdateBorrower(ctx context.Context, id int64, in BorrowerInput) (*Borrower, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetBorrower(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if other, err := q.BorrowerNamed(ctx, in.Name); err == nil && other != id {
			return &ConflictError{fmt.Sprintf("Another borrower is already named %s.", in.Name)}
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return q.UpdateBorrower(ctx, db.UpdateBorrowerParams{Name: in.Name, Contact: in.Contact, Notes: in.Notes, ID: id})
	})
	if err != nil {
		return nil, err
	}
	b, err := s.read().GetBorrower(ctx, id)
	if err != nil {
		return nil, err
	}
	out := borrowerOf(b)
	return &out, nil
}

// DeleteBorrower removes a borrower and their loans from the history. While they have a copy,
// it is refused: the copy has to come back first.
func (s *Service) DeleteBorrower(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		b, err := q.GetBorrower(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if b.OutCount > 0 {
			return &ConflictError{fmt.Sprintf("%s still has %s. Mark %s returned first.", b.Name, copiesWord(b.OutCount), itThem(b.OutCount))}
		}
		return q.DeleteBorrower(ctx, id)
	})
}

func borrowerOf(b *db.GetBorrowerRow) Borrower {
	return Borrower{
		ID:            b.ID,
		BorrowerInput: BorrowerInput{Name: b.Name, Contact: b.Contact, Notes: b.Notes},
		Out:           b.OutCount, Loans: b.LoanCount,
	}
}

// notLent refuses to remove an edition, or one copy of it, that someone has.
func notLent(ctx context.Context, q *db.Queries, editionID, copyID int64) error {
	if copyID > 0 {
		name, err := q.CopyOutTo(ctx, db.CopyOutToParams{CopyID: copyID})
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return &ConflictError{fmt.Sprintf("%s is lent to %s. Mark it returned before removing it.", Barcode(copyID), name)}
	}
	out, err := q.EditionOutTo(ctx, editionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return &ConflictError{fmt.Sprintf("%s is lent to %s. Mark it returned before removing the book.", Barcode(out.CopyID), out.Name)}
}

func copiesWord(n int64) string {
	if n == 1 {
		return "a copy"
	}
	return fmt.Sprintf("%d copies", n)
}

func itThem(n int64) string {
	if n == 1 {
		return "it"
	}
	return "them"
}
