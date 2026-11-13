package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"homelibrary/internal/db"
	"homelibrary/internal/labels"
)

// Bookcase is a bookcase with its shelves from the top down.
type Bookcase struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Position int64   `json:"position" doc:"In walking order, 1 up"`
	Shelves  []Shelf `json:"shelves"`
}

// NewBookcase is a bookcase as it is first measured: its shelves from the top down, each with
// its own measurements, since a bottom shelf is often taller than the rest.
type NewBookcase struct {
	Name    string       `json:"name" minLength:"1" maxLength:"100"`
	Shelves []ShelfInput `json:"shelves" minItems:"1" maxItems:"20" doc:"From the top down"`
}

// BookcaseInput renames a bookcase or moves it in the walking order.
type BookcaseInput struct {
	Name     string `json:"name" minLength:"1" maxLength:"100"`
	Position int64  `json:"position" minimum:"1" doc:"Where it comes in walking order"`
}

// ShelfPlace is where a copy stands in the layout in use.
type ShelfPlace struct {
	ShelfID  int64  `json:"shelfId"`
	Bookcase string `json:"bookcase"`
	Position int64  `json:"position" doc:"The shelf, from the top"`
	Area     string `json:"area" doc:"The area the shelf is in"`
	After    string `json:"after" doc:"The title of the book before it on the shelf; empty at the start"`
	Before   string `json:"before" doc:"The title of the book after it; empty at the end"`
	InUse    bool   `json:"inUse" doc:"Whether a layout is in use at all; with none, no book has a place"`
	Placed   bool   `json:"placed" doc:"Whether the layout in use has a place for it"`
	Shelves  bool   `json:"shelves" doc:"Whether any shelves are set up at all"`
}

func (in *ShelfInput) check() error {
	if in.WidthMM < 100 || in.HeightMM < 50 || in.DepthMM < 50 {
		return &FieldError{"widthMm", "measure the shelf inside its sides: at least 10 cm wide and 5 cm tall and deep"}
	}
	// A brace is both a height and a width, or neither.
	if in.BraceHeightMM == 0 || in.BraceWidthMM == 0 {
		in.BraceHeightMM, in.BraceWidthMM = 0, 0
	}
	if in.BraceHeightMM >= in.HeightMM || in.BraceWidthMM*2 >= in.WidthMM {
		return &FieldError{"braceHeightMm", "a corner brace has to be shorter than the shelf's height and narrower than half its width"}
	}
	return nil
}

func shelfParams(id int64, in ShelfInput) db.UpdateShelfParams {
	return db.UpdateShelfParams{WidthMm: in.WidthMM, HeightMm: in.HeightMM, DepthMm: in.DepthMM,
		BraceHeightMm: in.BraceHeightMM, BraceWidthMm: in.BraceWidthMM, ID: id}
}

func newShelf(bookcaseID, position int64, in ShelfInput) db.CreateShelfParams {
	return db.CreateShelfParams{BookcaseID: bookcaseID, Position: position, WidthMm: in.WidthMM, HeightMm: in.HeightMM, DepthMm: in.DepthMM,
		BraceHeightMm: in.BraceHeightMM, BraceWidthMm: in.BraceWidthMM}
}

// Bookcases lists the bookcases in walking order with their shelves.
func (s *Service) Bookcases(ctx context.Context) ([]Bookcase, error) {
	q := s.read()
	cases, err := q.ListBookcases(ctx)
	if err != nil {
		return nil, err
	}
	shelves, err := q.ListShelves(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Bookcase, 0, len(cases))
	for _, c := range cases {
		b := Bookcase{ID: c.ID, Name: c.Name, Position: c.Position, Shelves: []Shelf{}}
		for _, sh := range shelves {
			if sh.BookcaseID == c.ID {
				b.Shelves = append(b.Shelves, shelfOf(sh))
			}
		}
		out = append(out, b)
	}
	return out, nil
}

func shelfOf(r *db.ListShelvesRow) Shelf {
	return Shelf{ID: r.ID, BookcaseID: r.BookcaseID, Bookcase: r.Bookcase, Position: r.Position, AreaID: r.AreaID, Area: r.Area,
		ShelfInput: ShelfInput{WidthMM: r.WidthMm, HeightMM: r.HeightMm, DepthMM: r.DepthMm,
			BraceHeightMM: r.BraceHeightMm, BraceWidthMM: r.BraceWidthMm}}
}

// CreateBookcase adds a bookcase at the end of the walk, with its shelves.
func (s *Service) CreateBookcase(ctx context.Context, in NewBookcase) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return 0, &FieldError{"name", "a bookcase needs a name, like Living room, left"}
	}
	for i := range in.Shelves {
		if err := in.Shelves[i].check(); err != nil {
			return 0, nthShelf(i, err)
		}
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		if id, err = q.CreateBookcase(ctx, in.Name); err != nil {
			return fmt.Errorf("create bookcase: %w", err)
		}
		for n, shelf := range in.Shelves {
			if _, err := q.CreateShelf(ctx, newShelf(id, int64(n+1), shelf)); err != nil {
				return fmt.Errorf("create shelf: %w", err)
			}
		}
		return nil
	})
	return id, err
}

// nthShelf says which of a new bookcase's shelves a measurement is wrong on, counted from the top.
func nthShelf(i int, err error) error {
	var field *FieldError
	if !errors.As(err, &field) {
		return err
	}
	return &FieldError{fmt.Sprintf("shelves[%d].%s", i, field.Field), fmt.Sprintf("Shelf %d: %s", i+1, field.Message)}
}

// UpdateBookcase renames a bookcase and puts it at a place in the walk, the others moving up
// or down to make room.
func (s *Service) UpdateBookcase(ctx context.Context, id int64, in BookcaseInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return &FieldError{"name", "a bookcase needs a name"}
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		n, err := q.RenameBookcase(ctx, db.RenameBookcaseParams{Name: in.Name, ID: id})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		cases, err := q.ListBookcases(ctx)
		if err != nil {
			return err
		}
		order := make([]int64, 0, len(cases))
		for _, c := range cases {
			if c.ID != id {
				order = append(order, c.ID)
			}
		}
		at := min(max(int(in.Position)-1, 0), len(order))
		order = append(order[:at], append([]int64{id}, order[at:]...)...)
		for i, cid := range order {
			if err := q.SetBookcasePosition(ctx, db.SetBookcasePositionParams{Position: int64(i + 1), ID: cid}); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteBookcase removes a bookcase and its shelves; the books are only replanned.
func (s *Service) DeleteBookcase(ctx context.Context, id int64) error {
	n, err := db.New(s.DB.Write).DeleteBookcase(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddShelf adds a shelf at the bottom of a bookcase.
func (s *Service) AddShelf(ctx context.Context, bookcaseID int64, in ShelfInput) error {
	if err := in.check(); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		cases, err := q.ListBookcases(ctx)
		if err != nil {
			return err
		}
		found := false
		for _, c := range cases {
			found = found || c.ID == bookcaseID
		}
		if !found {
			return ErrNotFound
		}
		pos, err := q.NextShelfPosition(ctx, bookcaseID)
		if err != nil {
			return err
		}
		_, err = q.CreateShelf(ctx, newShelf(bookcaseID, pos, in))
		return err
	})
}

// UpdateShelf changes a shelf's measurements.
func (s *Service) UpdateShelf(ctx context.Context, id int64, in ShelfInput) error {
	if err := in.check(); err != nil {
		return err
	}
	n, err := db.New(s.DB.Write).UpdateShelf(ctx, shelfParams(id, in))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteShelf removes a shelf; the shelves below it move up a number.
func (s *Service) DeleteShelf(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		sh, err := q.GetShelf(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := q.DeleteShelf(ctx, id); err != nil {
			return err
		}
		all, err := q.ListShelves(ctx)
		if err != nil {
			return err
		}
		n := int64(0)
		for _, r := range all {
			if r.BookcaseID == sh.BookcaseID {
				n++
				if err := q.RenumberShelf(ctx, db.RenumberShelfParams{Position: n, ID: r.ID}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Layout returns the library on its shelves: the layout in use, or a suggestion when one is
// asked for. With neither, the shelves come back with nothing on them: no book is given a
// place until the owner asks.
func (s *Service) Layout(ctx context.Context, suggest bool) (*Layout, error) {
	q := s.read()
	settings, err := q.GetLayoutSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListShelves(ctx)
	if err != nil {
		return nil, err
	}
	areas, err := areasOf(ctx, q)
	if err != nil {
		return nil, err
	}
	out := &Layout{FillPercent: settings.FillPercent, AppliedAt: settings.AppliedAt, Shelves: make([]ShelfPlan, 0, len(rows)),
		NoArea: []ShelfBook{}, Unplaced: []ShelfBook{}, TooTall: []ShelfBook{}}
	starts := map[int64]areaKey{}
	for _, r := range rows {
		out.Shelves = append(out.Shelves, ShelfPlan{Shelf: shelfOf(r)})
		if i := slices.IndexFunc(areas, func(a Area) bool { return a.ID == r.AreaID }); i >= 0 {
			if k, ok := startFrom(r.StartsAt, &areas[i]); ok {
				starts[r.ID] = k
			}
		}
	}
	inUse := settings.AppliedAt != "" && len(starts) > 0
	if !suggest && !inUse {
		finish(out.Shelves, areas)
		return out, nil
	}
	copies, err := q.ListShelfCopies(ctx)
	if err != nil {
		return nil, err
	}
	authors, err := editionAuthors(ctx, q)
	if err != nil {
		return nil, err
	}
	books := shelfBooks(copies, authors)
	for _, b := range books {
		if b.Estimated {
			out.Estimated++
		}
	}
	byArea, noArea := sorted(areas, books)
	out.NoArea = append(out.NoArea, noArea...)
	var unplaced, tooTall []ShelfBook
	if suggest {
		out.Suggested = true
		unplaced, tooTall = propose(out.Shelves, areas, byArea, settings.FillPercent)
	} else {
		out.InUse = true
		for i := range out.Shelves {
			_, out.Shelves[i].InUse = starts[out.Shelves[i].ID]
		}
		unplaced, tooTall = hold(out.Shelves, starts, areas, byArea)
	}
	out.Unplaced, out.TooTall = append(out.Unplaced, unplaced...), append(out.TooTall, tooTall...)
	finish(out.Shelves, areas)
	return out, nil
}

// editionAuthors is who wrote each edition, by person, for the areas that take an author's books.
func editionAuthors(ctx context.Context, q *db.Queries) (map[int64][]int64, error) {
	rows, err := q.ListEditionAuthorIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("authors: %w", err)
	}
	out := map[int64][]int64{}
	for _, r := range rows {
		out[r.EditionID] = append(out[r.EditionID], r.PersonID)
	}
	return out, nil
}

// UseLayout puts a fresh suggestion to use: each shelf keeps where its first book falls in its
// area's order, and from then on a new book goes to the shelf of its area it belongs on, into
// the room the suggestion left, rather than every later book moving along.
func (s *Service) UseLayout(ctx context.Context) error {
	plan, err := s.Layout(ctx, true)
	if err != nil {
		return err
	}
	areas, err := s.Areas(ctx)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		if err := q.ClearShelfStarts(ctx); err != nil {
			return err
		}
		used := false
		for _, p := range plan.Shelves {
			i := slices.IndexFunc(areas, func(a Area) bool { return a.ID == p.AreaID })
			if len(p.Books) == 0 || i < 0 {
				continue
			}
			used = true
			if err := q.SetShelfStart(ctx, db.SetShelfStartParams{StartsAt: startOf(&areas[i], p.Books[0]), ID: p.ID}); err != nil {
				return err
			}
		}
		if !used {
			return &ConflictError{"No shelf has a book on it in the suggestion: give the areas shelves, and rules that take your books, first."}
		}
		return q.SetLayoutApplied(ctx, now())
	})
}

// StopLayout lets the shelves go: the planner proposes again from scratch.
func (s *Service) StopLayout(ctx context.Context) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		if err := q.ClearShelfStarts(ctx); err != nil {
			return err
		}
		return q.SetLayoutApplied(ctx, "")
	})
}

// SetFillPercent sets how full the planner fills a shelf.
func (s *Service) SetFillPercent(ctx context.Context, percent int64) error {
	if percent < 50 || percent > 100 {
		return &FieldError{"fillPercent", "fill a shelf between half and all the way"}
	}
	return db.New(s.DB.Write).SetFillPercent(ctx, percent)
}

// PlaceOf says where a copy stands in the layout in use. With no layout in use, no copy has a
// place: a suggestion is only ever the owner's to look at.
func (s *Service) PlaceOf(ctx context.Context, copyID int64) (*ShelfPlace, error) {
	q := s.read()
	if _, err := q.GetCopy(ctx, copyID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	l, err := s.Layout(ctx, false)
	if err != nil {
		return nil, err
	}
	place := &ShelfPlace{InUse: l.InUse, Shelves: len(l.Shelves) > 0}
	for _, p := range l.Shelves {
		for i, b := range p.Books {
			if b.CopyID != copyID {
				continue
			}
			place.ShelfID, place.Bookcase, place.Position, place.Area, place.Placed = p.ID, p.Bookcase, p.Position, p.Area, true
			if i > 0 {
				place.After = p.Books[i-1].Title
			}
			if i < len(p.Books)-1 {
				place.Before = p.Books[i+1].Title
			}
			return place, nil
		}
	}
	return place, nil
}

// RangeLabelsPDF draws a label for the edge of every shelf with books on it: where it is, and
// the first and last call numbers on it. They print on the spine-label sheet.
func (s *Service) RangeLabelsPDF(ctx context.Context) ([]byte, error) {
	l, err := s.Layout(ctx, false)
	if err != nil {
		return nil, err
	}
	if !l.InUse {
		return nil, &FieldError{"shelves", "no layout is in use: suggest one and put it to use first"}
	}
	var items []labels.Range
	for _, p := range l.Shelves {
		if len(p.Books) == 0 {
			continue
		}
		where := fmt.Sprintf("%s · shelf %d", p.Bookcase, p.Position)
		if p.Area != "" {
			where += " · " + p.Area
		}
		items = append(items, labels.Range{Where: where, First: p.First, Last: p.Last})
	}
	if len(items) == 0 {
		return nil, &FieldError{"shelves", "no shelf has a book on it yet: add bookcases and shelves first"}
	}
	off, err := s.offset(ctx, labels.Spine.Kind)
	if err != nil {
		return nil, err
	}
	return labels.Ranges(labels.Spine, items, off), nil
}
