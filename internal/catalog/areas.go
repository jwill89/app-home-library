package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// Areas are groups of shelves, each with rules for the books it takes and the order they stand
// in. A book goes to the first area, in the owner's order, whose rules take it, and an area
// fills its shelves in walking order. They need not stand side by side: the bottom shelf of
// every bookcase can be one area, for the tall books.

// AreaKeys are what an area can order its books by.
var AreaKeys = []string{"subject", "author", "series", "title", "published", "height"}

// defaultOrder is the order an area is given when none is chosen: the call numbers' own, by
// subject (the Dewey number), then author, series, and title.
var defaultOrder = []string{"subject", "author", "series", "title"}

// AreaRules are which books an area takes. Every rule set must hold, and any one entry of a list.
type AreaRules struct {
	Categories  []string `json:"categories" maxItems:"6" enum:"fiction,nonfiction,manga,light_novel,graphic_novel,uncategorized" doc:"Any of these categories; none for every one"`
	DeweyFrom   string   `json:"deweyFrom" maxLength:"20" doc:"The lowest Dewey number it takes, the book's own or estimated from its LC class; empty for no limit"`
	DeweyTo     string   `json:"deweyTo" maxLength:"20" doc:"The highest Dewey number it takes; empty for no limit"`
	Subject     string   `json:"subject" maxLength:"100" doc:"Words one of a book's subject headings has; empty for any"`
	Authors     []string `json:"authors" maxItems:"50" doc:"Books by any of these people; none for every author"`
	Series      []string `json:"series" maxItems:"50" doc:"Books in any of these series; none for every book"`
	MinHeightMM int64    `json:"minHeightMm" minimum:"0" maximum:"1000" doc:"Books at least this tall; 0 for no limit"`
	MaxHeightMM int64    `json:"maxHeightMm" minimum:"0" maximum:"1000" doc:"Books no taller than this; 0 for no limit"`
}

// AreaInput is an area as the owner sets it up.
type AreaInput struct {
	Name     string `json:"name" minLength:"1" maxLength:"100"`
	Position int64  `json:"position" minimum:"0" doc:"Where it comes in the order books try the areas, 1 first; 0 keeps its place, or puts a new one last"`
	AreaRules
	Order    []string `json:"order" maxItems:"6" enum:"subject,author,series,title,published,height" doc:"What its books are ordered by, first key first; none for subject, author, series, then title"`
	ShelfIDs []int64  `json:"shelfIds" maxItems:"500" doc:"Its shelves, which leave any other area"`
}

// Area is an area with its rules and its shelves in walking order.
type Area struct {
	ID int64 `json:"id"`
	AreaInput
	authorIDs []int64
	seriesIDs []int64
}

// Areas lists the areas in the order books try them.
func (s *Service) Areas(ctx context.Context) ([]Area, error) {
	return areasOf(ctx, s.read())
}

func areasOf(ctx context.Context, q *db.Queries) ([]Area, error) {
	rows, err := q.ListAreas(ctx)
	if err != nil {
		return nil, fmt.Errorf("areas: %w", err)
	}
	rules, err := q.ListAreaRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("area rules: %w", err)
	}
	shelves, err := q.ListShelves(ctx)
	if err != nil {
		return nil, fmt.Errorf("shelves: %w", err)
	}
	out := make([]Area, 0, len(rows))
	at := map[int64]int{}
	for _, r := range rows {
		at[r.ID] = len(out)
		out = append(out, Area{ID: r.ID, AreaInput: AreaInput{
			Name: r.Name, Position: r.Position, Order: []string{}, ShelfIDs: []int64{},
			AreaRules: AreaRules{
				Categories: []string{}, DeweyFrom: r.DeweyFrom, DeweyTo: r.DeweyTo, Subject: r.Subject,
				Authors: []string{}, Series: []string{}, MinHeightMM: r.MinHeightMm, MaxHeightMM: r.MaxHeightMm,
			},
		}})
	}
	for _, r := range rules {
		i, ok := at[r.AreaID]
		if !ok {
			continue
		}
		a := &out[i]
		switch r.Kind {
		case "category":
			a.Categories = append(a.Categories, r.Name)
		case "author":
			a.Authors, a.authorIDs = append(a.Authors, r.Name), append(a.authorIDs, r.Ref)
		case "series":
			a.Series, a.seriesIDs = append(a.Series, r.Name), append(a.seriesIDs, r.Ref)
		case "order":
			a.Order = append(a.Order, r.Name)
		}
	}
	for _, sh := range shelves {
		if i, ok := at[sh.AreaID]; ok {
			out[i].ShelfIDs = append(out[i].ShelfIDs, sh.ID)
		}
	}
	return out, nil
}

// CreateArea adds an area, last in the order books try them unless given a place.
func (s *Service) CreateArea(ctx context.Context, in AreaInput) (int64, error) {
	if err := in.check(); err != nil {
		return 0, err
	}
	var id int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		if err := uniqueAreaName(ctx, q, 0, in.Name); err != nil {
			return err
		}
		id, err = q.CreateArea(ctx, db.CreateAreaParams{Name: in.Name, DeweyFrom: in.DeweyFrom, DeweyTo: in.DeweyTo,
			Subject: in.Subject, MinHeightMm: in.MinHeightMM, MaxHeightMm: in.MaxHeightMM})
		if err != nil {
			return fmt.Errorf("create area: %w", err)
		}
		return writeArea(ctx, q, id, in)
	})
	return id, err
}

// UpdateArea changes an area's name, place, rules, order, and shelves.
func (s *Service) UpdateArea(ctx context.Context, id int64, in AreaInput) error {
	if err := in.check(); err != nil {
		return err
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		if err := uniqueAreaName(ctx, q, id, in.Name); err != nil {
			return err
		}
		n, err := q.UpdateArea(ctx, db.UpdateAreaParams{Name: in.Name, DeweyFrom: in.DeweyFrom, DeweyTo: in.DeweyTo,
			Subject: in.Subject, MinHeightMm: in.MinHeightMM, MaxHeightMm: in.MaxHeightMM, ID: id})
		if err != nil {
			return fmt.Errorf("update area: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return writeArea(ctx, q, id, in)
	})
}

// DeleteArea removes an area; its shelves are left in none.
func (s *Service) DeleteArea(ctx context.Context, id int64) error {
	n, err := db.New(s.DB.Write).DeleteArea(ctx, id)
	if err != nil {
		return fmt.Errorf("delete area: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// check tidies an area's input and says what is wrong with it, by field.
func (in *AreaInput) check() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return &FieldError{"name", "an area needs a name, like Nonfiction or Tall books"}
	}
	in.Subject = strings.TrimSpace(in.Subject)
	for _, f := range []struct {
		field string
		v     *string
	}{{"deweyFrom", &in.DeweyFrom}, {"deweyTo", &in.DeweyTo}} {
		if strings.TrimSpace(*f.v) == "" {
			*f.v = ""
			continue
		}
		if *f.v = lookup.NormalizeDDC(*f.v); *f.v == "" {
			return &FieldError{f.field, "a Dewey number is three digits and any decimals, like 500 or 599.9"}
		}
	}
	if in.DeweyFrom != "" && in.DeweyTo != "" && deweyValue(in.DeweyFrom) > deweyValue(in.DeweyTo) {
		return &FieldError{"deweyTo", "the Dewey range has to end at or after where it starts"}
	}
	if in.MinHeightMM > 0 && in.MaxHeightMM > 0 && in.MinHeightMM > in.MaxHeightMM {
		return &FieldError{"maxHeightMm", "the tallest has to be at least the shortest"}
	}
	in.Categories = cleanList(in.Categories)
	in.Authors = cleanList(in.Authors)
	in.Series = cleanList(in.Series)
	if len(in.Order) == 0 {
		in.Order = slices.Clone(defaultOrder)
	}
	seen := map[string]bool{}
	for _, k := range in.Order {
		if seen[k] {
			return &FieldError{"order", "each key orders the books once"}
		}
		seen[k] = true
	}
	slices.Sort(in.ShelfIDs)
	in.ShelfIDs = slices.Compact(in.ShelfIDs)
	return nil
}

func uniqueAreaName(ctx context.Context, q *db.Queries, id int64, name string) error {
	areas, err := q.ListAreas(ctx)
	if err != nil {
		return fmt.Errorf("areas: %w", err)
	}
	for _, a := range areas {
		if a.ID != id && strings.EqualFold(a.Name, name) {
			return &FieldError{"name", "another area is called that"}
		}
	}
	return nil
}

// writeArea puts an area's lists, shelves, and place as the input has them, the names of its
// authors and series matched to the library's.
func writeArea(ctx context.Context, q *db.Queries, id int64, in AreaInput) error {
	for _, empty := range []func(context.Context, int64) error{q.ClearAreaCategories, q.ClearAreaAuthors, q.ClearAreaSeries, q.ClearAreaOrder} {
		if err := empty(ctx, id); err != nil {
			return fmt.Errorf("clear area: %w", err)
		}
	}
	if err := q.ClearAreaShelves(ctx, sql.NullInt64{Int64: id, Valid: true}); err != nil {
		return fmt.Errorf("clear area shelves: %w", err)
	}
	for _, c := range in.Categories {
		if err := q.AddAreaCategory(ctx, db.AddAreaCategoryParams{AreaID: id, Category: c}); err != nil {
			return fmt.Errorf("area category: %w", err)
		}
	}
	for _, name := range in.Authors {
		p, err := q.GetPersonByName(ctx, name)
		if errors.Is(err, sql.ErrNoRows) {
			return &FieldError{"authors", fmt.Sprintf("the library has no one called %s", name)}
		} else if err != nil {
			return fmt.Errorf("author: %w", err)
		}
		if err := q.AddAreaAuthor(ctx, db.AddAreaAuthorParams{AreaID: id, PersonID: p.ID}); err != nil {
			return fmt.Errorf("area author: %w", err)
		}
	}
	all, err := q.ListSeries(ctx)
	if err != nil {
		return fmt.Errorf("series: %w", err)
	}
	for _, name := range in.Series {
		se := matchSeries(all, name)
		if se == nil {
			return &FieldError{"series", fmt.Sprintf("the library has no series called %s", name)}
		}
		if err := q.AddAreaSeries(ctx, db.AddAreaSeriesParams{AreaID: id, SeriesID: se.ID}); err != nil {
			return fmt.Errorf("area series: %w", err)
		}
	}
	for i, k := range in.Order {
		if err := q.AddAreaOrder(ctx, db.AddAreaOrderParams{AreaID: id, Rank: int64(i + 1), Key: k}); err != nil {
			return fmt.Errorf("area order: %w", err)
		}
	}
	for _, sh := range in.ShelfIDs {
		n, err := q.SetShelfArea(ctx, db.SetShelfAreaParams{AreaID: sql.NullInt64{Int64: id, Valid: true}, ID: sh})
		if err != nil {
			return fmt.Errorf("shelf area: %w", err)
		}
		if n == 0 {
			return &FieldError{"shelfIds", "one of those shelves has been removed: open the area again"}
		}
	}
	if in.Position == 0 {
		return nil
	}
	areas, err := q.ListAreas(ctx)
	if err != nil {
		return fmt.Errorf("areas: %w", err)
	}
	order := make([]int64, 0, len(areas))
	for _, a := range areas {
		if a.ID != id {
			order = append(order, a.ID)
		}
	}
	at := min(max(int(in.Position)-1, 0), len(order))
	order = slices.Insert(order, at, id)
	for i, aid := range order {
		if err := q.SetAreaPosition(ctx, db.SetAreaPositionParams{Position: int64(i + 1), ID: aid}); err != nil {
			return fmt.Errorf("area position: %w", err)
		}
	}
	return nil
}

// deweyValue is a Dewey number as a number, for comparing ranges and ordering by subject.
func deweyValue(ddc string) float64 {
	v, err := strconv.ParseFloat(ddc, 64)
	if err != nil {
		return 0
	}
	return v
}
