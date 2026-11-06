package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// NewBookcase is a bookcase as it is first measured: its shelves all alike, as most are.
type NewBookcase struct {
	Name    string `json:"name" minLength:"1" maxLength:"100"`
	Shelves int64  `json:"shelves" minimum:"1" maximum:"20"`
	ShelfInput
}

// BookcaseInput renames a bookcase or moves it in the walking order.
type BookcaseInput struct {
	Name     string `json:"name" minLength:"1" maxLength:"100"`
	Position int64  `json:"position" minimum:"1" doc:"Where it comes in walking order"`
}

// ShelfPlace is where a copy stands.
type ShelfPlace struct {
	ShelfID  int64  `json:"shelfId"`
	Bookcase string `json:"bookcase"`
	Position int64  `json:"position" doc:"The shelf, from the top"`
	Oversize bool   `json:"oversize"`
	After    string `json:"after" doc:"The title of the book before it on the shelf; empty at the start"`
	Before   string `json:"before" doc:"The title of the book after it; empty at the end"`
	InUse    bool   `json:"inUse" doc:"From the layout in use; false is where a proposal would put it"`
	Placed   bool   `json:"placed" doc:"Whether any shelf has room for it"`
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
	return db.UpdateShelfParams{WidthMm: in.WidthMM, HeightMm: in.HeightMM, DepthMm: in.DepthMM, Oversize: boolInt(in.Oversize),
		BraceHeightMm: in.BraceHeightMM, BraceWidthMm: in.BraceWidthMM, ID: id}
}

func newShelf(bookcaseID, position int64, in ShelfInput) db.CreateShelfParams {
	return db.CreateShelfParams{BookcaseID: bookcaseID, Position: position, WidthMm: in.WidthMM, HeightMm: in.HeightMM, DepthMm: in.DepthMM,
		Oversize: boolInt(in.Oversize), BraceHeightMm: in.BraceHeightMM, BraceWidthMm: in.BraceWidthMM}
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
	return Shelf{ID: r.ID, BookcaseID: r.BookcaseID, Bookcase: r.Bookcase, Position: r.Position,
		ShelfInput: ShelfInput{WidthMM: r.WidthMm, HeightMM: r.HeightMm, DepthMM: r.DepthMm, Oversize: r.Oversize == 1,
			BraceHeightMM: r.BraceHeightMm, BraceWidthMM: r.BraceWidthMm}}
}

// CreateBookcase adds a bookcase at the end of the walk, with its shelves.
func (s *Service) CreateBookcase(ctx context.Context, in NewBookcase) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return 0, &FieldError{"name", "a bookcase needs a name, like Living room, left"}
	}
	if err := in.check(); err != nil {
		return 0, err
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		if id, err = q.CreateBookcase(ctx, in.Name); err != nil {
			return fmt.Errorf("create bookcase: %w", err)
		}
		for n := range in.Shelves {
			if _, err := q.CreateShelf(ctx, newShelf(id, n+1, in.ShelfInput)); err != nil {
				return fmt.Errorf("create shelf: %w", err)
			}
		}
		return nil
	})
	return id, err
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

// UpdateShelf changes a shelf's measurements or whether it is for oversize books.
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

// Layout returns the library on its shelves: the layout in use when there is one and a
// proposal is not asked for, otherwise a fresh proposal.
func (s *Service) Layout(ctx context.Context, proposal bool) (*Layout, error) {
	q := s.read()
	settings, err := q.GetLayoutSettings(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := q.ListShelves(ctx)
	if err != nil {
		return nil, err
	}
	copies, err := q.ListShelfCopies(ctx)
	if err != nil {
		return nil, err
	}
	books := shelfBooks(copies)
	out := &Layout{FillPercent: settings.FillPercent, AppliedAt: settings.AppliedAt, Shelves: make([]ShelfPlan, 0, len(rows))}
	starts := map[int64]*db.CopyShelving{}
	for _, r := range rows {
		out.Shelves = append(out.Shelves, ShelfPlan{Shelf: shelfOf(r)})
		if k := parseStart(r.StartsAt); k != nil {
			starts[r.ID] = k
		}
	}
	for _, b := range books {
		if b.Estimated {
			out.Estimated++
		}
	}
	out.InUse = settings.AppliedAt != "" && len(starts) > 0 && !proposal
	if out.InUse {
		for i := range out.Shelves {
			_, out.Shelves[i].InUse = starts[out.Shelves[i].ID]
		}
		out.Unplaced, out.TooTall = hold(out.Shelves, starts, books, settings.FillPercent)
	} else {
		out.Unplaced, out.TooTall = propose(out.Shelves, books, settings.FillPercent)
	}
	finish(out.Shelves)
	if out.Unplaced == nil {
		out.Unplaced = []ShelfBook{}
	}
	if out.TooTall == nil {
		out.TooTall = []ShelfBook{}
	}
	return out, nil
}

// UseLayout puts the proposal to use: each shelf keeps where its first book falls in shelf
// order, and from then on a new book goes to the shelf its call number belongs on, into the
// room the proposal left, rather than every later book moving along.
func (s *Service) UseLayout(ctx context.Context) error {
	plan, err := s.Layout(ctx, true)
	if err != nil {
		return err
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		if err := q.ClearShelfStarts(ctx); err != nil {
			return err
		}
		used := false
		for _, p := range plan.Shelves {
			if len(p.Books) == 0 {
				continue
			}
			used = true
			if err := q.SetShelfStart(ctx, db.SetShelfStartParams{StartsAt: startOf(p.Books[0]), ID: p.ID}); err != nil {
				return err
			}
		}
		if !used {
			return &ConflictError{"No shelf has a book on it in the proposal: add bookcases and shelves first."}
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

// PlaceOf says where a copy stands: on its shelf in the layout in use, or where a proposal
// would put it.
func (s *Service) PlaceOf(ctx context.Context, copyID int64) (*ShelfPlace, error) {
	q := s.read()
	if _, err := q.GetCopy(ctx, copyID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	// With no shelves there is nowhere to stand, and no layout worth working out to say so.
	if shelves, err := q.ListShelves(ctx); err != nil {
		return nil, err
	} else if len(shelves) == 0 {
		return &ShelfPlace{}, nil
	}
	l, err := s.Layout(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, p := range l.Shelves {
		for i, b := range p.Books {
			if b.CopyID != copyID {
				continue
			}
			place := &ShelfPlace{ShelfID: p.ID, Bookcase: p.Bookcase, Position: p.Position, Oversize: p.Oversize, InUse: l.InUse, Placed: true, Shelves: true}
			if i > 0 {
				place.After = p.Books[i-1].Title
			}
			if i < len(p.Books)-1 {
				place.Before = p.Books[i+1].Title
			}
			return place, nil
		}
	}
	return &ShelfPlace{InUse: l.InUse, Shelves: len(l.Shelves) > 0}, nil
}

// RangeLabelsPDF draws a label for the edge of every shelf with books on it: where it is, and
// the first and last call numbers on it. They print on the spine-label sheet.
func (s *Service) RangeLabelsPDF(ctx context.Context) ([]byte, error) {
	l, err := s.Layout(ctx, false)
	if err != nil {
		return nil, err
	}
	var items []labels.Range
	for _, p := range l.Shelves {
		if len(p.Books) == 0 {
			continue
		}
		where := fmt.Sprintf("%s · shelf %d", p.Bookcase, p.Position)
		if p.Oversize {
			where += " · oversize"
		}
		// A book without a call number is named on the label by its title.
		first, last := strings.Join(p.First, " "), strings.Join(p.Last, " ")
		if first == "" {
			first = p.Books[0].Title
		}
		if last == "" {
			last = p.Books[len(p.Books)-1].Title
		}
		items = append(items, labels.Range{Where: where, First: first, Last: last})
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
