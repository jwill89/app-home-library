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

// Series is a series as the Series page shows it: the volumes had, the ones only wanted, the
// whole numbers missing among them, and the number that would come next.
type Series struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Books  int64    `json:"books" doc:"Volumes had, numbered or not"`
	Had    []string `json:"had" doc:"The numbers of the volumes had, in order"`
	Wanted []string `json:"wanted" doc:"The numbers of the volumes on the wishlist"`
	Gaps   []string `json:"gaps" doc:"Whole numbers from 1 to the highest had that are neither had nor wanted"`
	Next   string   `json:"next" doc:"The whole number after the highest had; empty when no volume is numbered"`
}

// gapLimit keeps a series numbered by year or by issue (2019, 450) from listing hundreds of
// gaps: past it, the gaps are not worked out.
const gapLimit = 200

// SeriesList returns every series the library has a volume of, by name. A share link's has
// only the volumes it shows, as its list of books does.
func (s *Service) SeriesList(ctx context.Context, sc *Scope) ([]Series, error) {
	rows, err := s.read().ListSeriesVolumes(ctx)
	if err != nil {
		return nil, fmt.Errorf("series: %w", err)
	}
	shown, err := s.shown(ctx, sc)
	if err != nil {
		return nil, fmt.Errorf("series: %w", err)
	}
	out := []Series{}
	for _, r := range rows {
		if shown != nil && !shown[r.EditionID] {
			continue
		}
		if len(out) == 0 || out[len(out)-1].ID != r.ID {
			out = append(out, Series{ID: r.ID, Name: r.Name, Had: []string{}, Wanted: []string{}, Gaps: []string{}})
		}
		se := &out[len(out)-1]
		n := strings.TrimSpace(r.SeriesNumber)
		switch {
		case r.WantedOn != "":
			if n != "" {
				se.Wanted = append(se.Wanted, n)
			}
		default:
			se.Books++
			if n != "" {
				se.Had = append(se.Had, n)
			}
		}
	}
	for i := range out {
		gaps(&out[i])
	}
	return out, nil
}

// gaps orders a series' numbers and works out the missing whole numbers and the next one.
func gaps(se *Series) {
	num := func(v string) float64 {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return -1
		}
		return f
	}
	byNumber := func(a, b string) int {
		if c := num(a) - num(b); c != 0 {
			if c < 0 {
				return -1
			}
			return 1
		}
		return strings.Compare(a, b)
	}
	se.Had = slices.Compact(slices.SortedFunc(slices.Values(se.Had), byNumber))
	se.Wanted = slices.Compact(slices.SortedFunc(slices.Values(se.Wanted), byNumber))
	high := 0
	known := map[int]bool{}
	for _, v := range slices.Concat(se.Had, se.Wanted) {
		f := num(v)
		if f >= 1 && f == float64(int(f)) {
			known[int(f)] = true
		}
	}
	for _, v := range se.Had {
		if f := num(v); f >= 1 {
			high = max(high, int(f))
		}
	}
	if high == 0 {
		return
	}
	se.Next = strconv.Itoa(high + 1)
	if high > gapLimit {
		return
	}
	for n := 1; n < high; n++ {
		if !known[n] {
			se.Gaps = append(se.Gaps, strconv.Itoa(n))
		}
	}
}

// SeriesInput is a series' new name.
type SeriesInput struct {
	Name string `json:"name" minLength:"1" maxLength:"200"`
}

// RenameSeries gives a series a new name. Given the name another series has, the two become
// one: this one's volumes join the other, which keeps its name. It returns the series that
// remains.
func (s *Service) RenameSeries(ctx context.Context, id int64, in SeriesInput) (int64, error) {
	name := strings.Join(strings.Fields(in.Name), " ")
	if name == "" {
		return 0, &FieldError{"name", "a series needs a name"}
	}
	keep := id
	err := s.inTx(ctx, func(q *db.Queries) error {
		all, err := q.ListSeries(ctx)
		if err != nil {
			return err
		}
		// The same name, written alike or another way, is the same series: this one merges
		// into it. One put aside as no series is a series again, the owner having named it.
		if other := matchSeries(all, name); other != nil && other.ID != id {
			if other.DismissedOn != "" {
				if err := q.SetSeriesDismissed(ctx, db.SetSeriesDismissedParams{DismissedOn: "", ID: other.ID}); err != nil {
					return fmt.Errorf("series taken back: %w", err)
				}
			}
			if err := q.MoveSeriesWorks(ctx, db.MoveSeriesWorksParams{Into: sql.NullInt64{Int64: other.ID, Valid: true}, FromID: sql.NullInt64{Int64: id, Valid: true}}); err != nil {
				return fmt.Errorf("merge series: %w", err)
			}
			keep = other.ID
			return q.DeleteSeries(ctx, id)
		}
		n, err := q.RenameSeries(ctx, db.RenameSeriesParams{Name: name, ID: id})
		if err != nil {
			return fmt.Errorf("rename series: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return nil
	})
	return keep, err
}

// GetSeries returns one series as the Series page shows it.
func (s *Service) GetSeries(ctx context.Context, id int64, sc *Scope) (*Series, error) {
	all, err := s.SeriesList(ctx, sc)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(all, func(se Series) bool { return se.ID == id })
	if i < 0 {
		return nil, ErrNotFound
	}
	return &all[i], nil
}

// LeaveSeries takes a book out of a series, and out of its number in it. The owner said so,
// so no lookup puts the book in a series again: the owner may still type one.
func (s *Service) LeaveSeries(ctx context.Context, seriesID, bookID int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		ed, err := q.GetEdition(ctx, bookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		n, err := q.LeaveSeries(ctx, db.LeaveSeriesParams{WorkID: ed.WorkID, SeriesID: nullInt(seriesID)})
		if err != nil {
			return fmt.Errorf("leave series: %w", err)
		}
		if n == 0 {
			return ErrNotFound // not in this series
		}
		return q.SetSeriesByOwner(ctx, ed.WorkID)
	})
}

// DismissSeries puts a series aside as no series at all (a publisher's line a source gave as
// one, such as Penguin classics): every book leaves it, and no lookup or import puts a book in
// it again. The books may be put in another series. It says how many books left.
func (s *Service) DismissSeries(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		var err error
		if n, err = q.EmptySeries(ctx, nullInt(id)); err != nil {
			return fmt.Errorf("empty series: %w", err)
		}
		if n == 0 {
			return ErrNotFound
		}
		return q.SetSeriesDismissed(ctx, db.SetSeriesDismissedParams{DismissedOn: today(), ID: id})
	})
	return n, err
}

// seriesKey is what a series' name comes to when its case, its punctuation, an article at
// either end ("The Broken Empire", "Raven's Blade Novel, A"), and a closing "series" are let
// go: two names with one key are one series, written two ways.
func seriesKey(name string) string {
	words := keyWords(name)
	if len(words) > 1 && isArticle(words[0]) {
		words = words[1:]
	}
	if len(words) > 1 && (isArticle(words[len(words)-1]) || words[len(words)-1] == "series") {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// matchSeries finds the series a name is: the one so named, else one written another way.
func matchSeries(all []*db.Series, name string) *db.Series {
	if i := slices.IndexFunc(all, func(se *db.Series) bool { return strings.EqualFold(se.Name, name) }); i >= 0 {
		return all[i]
	}
	key := seriesKey(name)
	if i := slices.IndexFunc(all, func(se *db.Series) bool { return seriesKey(se.Name) == key }); i >= 0 && key != "" {
		return all[i]
	}
	return nil
}

// fitSeries puts a lookup's series in the library's terms: the name the library writes it
// with, or none for a series the owner put aside, so it is neither filled nor offered.
func (s *Service) fitSeries(ctx context.Context, p *lookup.Proposal) error {
	if p.Series == "" {
		return nil
	}
	all, err := s.read().ListSeries(ctx)
	if err != nil {
		return fmt.Errorf("series: %w", err)
	}
	se := matchSeries(all, p.Series)
	switch {
	case se == nil:
	case se.DismissedOn != "":
		p.Series, p.SeriesNumber = "", ""
		delete(p.Sources, "series")
		delete(p.Sources, "seriesNumber")
	default:
		p.Series = se.Name
	}
	return nil
}
