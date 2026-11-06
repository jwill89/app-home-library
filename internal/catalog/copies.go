package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"homelibrary/internal/db"
)

// AddCopy records another physical copy of an edition and returns it.
func (s *Service) AddCopy(ctx context.Context, editionID int64, in CopyInput) (*Copy, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetEdition(ctx, editionID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var err error
		id, err = createCopy(ctx, q, editionID, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetCopy(ctx, id)
}

// UpdateCopy replaces a copy's details.
func (s *Service) UpdateCopy(ctx context.Context, id int64, in CopyInput) (*Copy, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCopy(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		section, err := upsertSection(ctx, q, in.Section)
		if err != nil {
			return err
		}
		return q.UpdateCopy(ctx, db.UpdateCopyParams{
			SectionID: section, AcquiredAs: storedChoice(in.AcquiredAs), AcquiredOn: in.AcquiredOn,
			AcquiredFrom: in.AcquiredFrom, PriceCents: price(in.PriceCents), Condition: storedChoice(in.Condition),
			DustJacket: jacket(in.DustJacket), Signed: boolInt(in.Signed), Location: in.Location, Notes: in.Notes, ID: id,
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCopy(ctx, id)
}

// DeleteCopy removes one copy. The edition stays, so a book given away keeps its record. A
// copy someone has cannot be removed until it is back.
func (s *Service) DeleteCopy(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetCopy(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := notLent(ctx, q, 0, id); err != nil {
			return err
		}
		return q.DeleteCopy(ctx, id)
	})
}

// GetCopy returns one copy.
func (s *Service) GetCopy(ctx context.Context, id int64) (*Copy, error) {
	q := s.read()
	c, err := q.GetCopy(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	all, err := s.Get(ctx, c.EditionID)
	if err != nil {
		return nil, err
	}
	for _, cp := range all.Copies {
		if cp.ID == id {
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

// withLoans puts each loan out on the copy it is of.
func (s *Service) withLoans(copies []Copy, loans []*db.LoanRow) {
	for _, l := range loans {
		for i := range copies {
			if copies[i].ID == l.CopyID {
				loan := s.loanOf(l)
				copies[i].OnLoan, copies[i].Loan = true, &loan
			}
		}
	}
}

// EditionOfCopy returns the edition a copy belongs to, for a scanned label.
func (s *Service) EditionOfCopy(ctx context.Context, copyID int64) (int64, error) {
	c, err := s.read().GetCopy(ctx, copyID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return c.EditionID, nil
}

func createCopy(ctx context.Context, q *db.Queries, editionID int64, in CopyInput) (int64, error) {
	section, err := upsertSection(ctx, q, in.Section)
	if err != nil {
		return 0, err
	}
	id, err := q.CreateCopy(ctx, db.CreateCopyParams{
		EditionID: editionID, SectionID: section, AcquiredAs: storedChoice(in.AcquiredAs), AcquiredOn: in.AcquiredOn,
		AcquiredFrom: in.AcquiredFrom, PriceCents: price(in.PriceCents), Condition: storedChoice(in.Condition),
		DustJacket: jacket(in.DustJacket), Signed: boolInt(in.Signed), Location: in.Location, Notes: in.Notes,
	})
	if err != nil {
		return 0, fmt.Errorf("create copy: %w", err)
	}
	if err := queueLabels(ctx, q, id); err != nil {
		return 0, err
	}
	// A book the owner has is not one they want.
	if err := q.SetEditionWanted(ctx, db.SetEditionWantedParams{WantedOn: "", ID: editionID}); err != nil {
		return 0, fmt.Errorf("off the wishlist: %w", err)
	}
	return id, nil
}

func upsertSection(ctx context.Context, q *db.Queries, name string) (sql.NullInt64, error) {
	if name == "" {
		return sql.NullInt64{}, nil
	}
	id, err := q.UpsertSection(ctx, name)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("section: %w", err)
	}
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

func copyOf(c *db.ListEditionCopiesRow) Copy {
	out := Copy{
		ID: c.ID, Barcode: Barcode(c.ID), CreatedAt: c.CreatedAt, CallNumber: []string{}, Labels: []string{},
		GoneAs: c.GoneAs, GoneOn: c.GoneOn, GoneNote: c.GoneNote, SeenOn: c.SeenOn,
		CopyInput: CopyInput{
			Section: c.SectionName, AcquiredAs: shownChoice(c.AcquiredAs), AcquiredOn: c.AcquiredOn,
			AcquiredFrom: c.AcquiredFrom, PriceCents: -1, Condition: shownChoice(c.Condition),
			DustJacket: "unknown", Signed: c.Signed == 1, Location: c.Location, Notes: c.Notes,
		},
	}
	if c.PriceCents.Valid {
		out.PriceCents = c.PriceCents.Int64
	}
	if c.DustJacket.Valid {
		out.DustJacket = map[int64]string{0: "no", 1: "yes"}[c.DustJacket.Int64]
	}
	return out
}

// The API says "unknown" where the database keeps an empty string, so every choice the
// form offers is a value of its own.
func storedChoice(s string) string {
	if s == "unknown" {
		return ""
	}
	return s
}

func shownChoice(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func price(cents int64) sql.NullInt64 {
	if cents < 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: cents, Valid: true}
}

func jacket(s string) sql.NullInt64 {
	switch s {
	case "yes":
		return sql.NullInt64{Int64: 1, Valid: true}
	case "no":
		return sql.NullInt64{Int64: 0, Valid: true}
	}
	return sql.NullInt64{}
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// GoneInput is how a copy left the library, and when.
type GoneInput struct {
	As   string `json:"as" enum:"lost,sold,donated,given,discarded"`
	On   string `json:"on" doc:"YYYY-MM-DD"`
	Note string `json:"note" maxLength:"500"`
}

// MarkGone records that a copy has left the library: it keeps its record and its history, and
// leaves the shelves, the label queue, and the counts. A copy out on loan comes back first.
func (s *Service) MarkGone(ctx context.Context, id int64, in GoneInput) (*Copy, error) {
	if !dayRe.MatchString(in.On) {
		return nil, &FieldError{"on", "give the day it went, as YYYY-MM-DD"}
	}
	err := s.inTx(ctx, func(q *db.Queries) error {
		name, err := q.CopyOutTo(ctx, db.CopyOutToParams{CopyID: id, ExceptLoan: 0})
		if err == nil {
			return &ConflictError{fmt.Sprintf("%s is lent to %s. Mark it returned first.", Barcode(id), name)}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		n, err := q.SetCopyGone(ctx, db.SetCopyGoneParams{GoneAs: in.As, GoneOn: in.On, GoneNote: strings.TrimSpace(in.Note), ID: id})
		if err != nil {
			return fmt.Errorf("mark gone: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return q.UnqueueCopyLabels(ctx, id)
	})
	if err != nil {
		return nil, err
	}
	return s.GetCopy(ctx, id)
}

// BringBack puts a copy that had gone back in the library, as one found again.
func (s *Service) BringBack(ctx context.Context, id int64) (*Copy, error) {
	n, err := db.New(s.DB.Write).SetCopyGone(ctx, db.SetCopyGoneParams{ID: id})
	if err != nil {
		return nil, fmt.Errorf("bring back: %w", err)
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetCopy(ctx, id)
}

// MarkSeen records the day a shelf check found a copy.
func (s *Service) MarkSeen(ctx context.Context, id int64, on string) error {
	if !dayRe.MatchString(on) {
		return &FieldError{"on", "give the day, as YYYY-MM-DD"}
	}
	n, err := db.New(s.DB.Write).SetCopySeen(ctx, db.SetCopySeenParams{SeenOn: on, ID: id})
	if err != nil {
		return fmt.Errorf("mark seen: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
