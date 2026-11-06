package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"homelibrary/internal/db"
)

// Reading is one earlier read of a book.
type Reading struct {
	ID         int64  `json:"id"`
	StartedOn  string `json:"startedOn"`
	FinishedOn string `json:"finishedOn"`
}

func readingsOf(rows []*db.Reading) []Reading {
	out := make([]Reading, 0, len(rows))
	for _, r := range rows {
		out = append(out, Reading{ID: r.ID, StartedOn: r.StartedOn, FinishedOn: r.FinishedOn})
	}
	return out
}

// ReadAgain begins a book again on the day given: the read it had, finished or not, is kept
// among its earlier reads first. A book being read now is begun already.
func (s *Service) ReadAgain(ctx context.Context, bookID int64, on string) error {
	if !dayRe.MatchString(on) {
		return &FieldError{"on", "give the day, as YYYY-MM-DD"}
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		ed, err := q.GetEdition(ctx, bookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		w, err := q.GetWork(ctx, ed.WorkID)
		if err != nil {
			return err
		}
		if w.ReadStatus == "reading" {
			return &ConflictError{"This book is being read now. Mark it read first."}
		}
		if w.ReadStatus == "read" || w.StartedOn != "" || w.FinishedOn != "" {
			if err := q.CreateReading(ctx, db.CreateReadingParams{WorkID: w.ID, StartedOn: w.StartedOn, FinishedOn: w.FinishedOn}); err != nil {
				return fmt.Errorf("keep the read: %w", err)
			}
		}
		return q.ReadAgain(ctx, db.ReadAgainParams{StartedOn: on, ID: w.ID})
	})
}

// ReadStatusInput is a book's reading status, changed on its own.
type ReadStatusInput struct {
	ReadStatus string `json:"readStatus" enum:"unread,reading,read"`
	On         string `json:"on" doc:"The day on the owner's device, as YYYY-MM-DD: the day a book begun now keeps, where it has none"`
}

// SetReadStatus changes a book's reading status. A book begun is begun on the day given, unless it
// keeps a day already: reading is now. A book marked read is given no day: it is as often one read
// years ago as one finished today, and the owner enters the day when it is known.
func (s *Service) SetReadStatus(ctx context.Context, bookID int64, in ReadStatusInput) error {
	if !dayRe.MatchString(in.On) {
		return &FieldError{"on", "give the day, as YYYY-MM-DD"}
	}
	return s.inTx(ctx, func(q *db.Queries) error {
		ed, err := q.GetEdition(ctx, bookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		w, err := q.GetWork(ctx, ed.WorkID)
		if err != nil {
			return err
		}
		started, finished := w.StartedOn, w.FinishedOn
		if in.ReadStatus == "reading" && started == "" {
			started = in.On
		}
		return q.SetReadStatus(ctx, db.SetReadStatusParams{ReadStatus: in.ReadStatus, StartedOn: started, FinishedOn: finished, ID: w.ID})
	})
}

// SeriesReadInput is a whole series marked read, or unread.
type SeriesReadInput struct {
	ReadStatus string `json:"readStatus" enum:"unread,read"`
}

// SetSeriesRead marks every book of a series the house has read or unread, and says how many
// changed. It records the past, so no day is made up: each book keeps the days it has.
func (s *Service) SetSeriesRead(ctx context.Context, seriesID int64, in SeriesReadInput) (int64, error) {
	n, err := db.New(s.DB.Write).SetSeriesReadStatus(ctx, db.SetSeriesReadStatusParams{
		Status: in.ReadStatus, SeriesID: nullInt(seriesID), OtherThan: in.ReadStatus,
	})
	if err != nil {
		return 0, fmt.Errorf("series %d read: %w", seriesID, err)
	}
	return n, nil
}

// DeleteReading removes an earlier read recorded by mistake.
func (s *Service) DeleteReading(ctx context.Context, id int64) error {
	n, err := db.New(s.DB.Write).DeleteReading(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReadingYear is what was finished in one year.
type ReadingYear struct {
	Year  string `json:"year"`
	Books int64  `json:"books" doc:"Reads finished, a book read twice counted twice"`
	Pages int64  `json:"pages" doc:"Their pages, where known"`
}

// FinishedRead is one read finished.
type FinishedRead struct {
	BookID     int64  `json:"bookId"`
	Title      string `json:"title"`
	FinishedOn string `json:"finishedOn"`
	Pages      int64  `json:"pages"`
}

// ReadingStats is the reading as the Reading page shows it.
type ReadingStats struct {
	Years    []ReadingYear  `json:"years" doc:"Newest first"`
	Finished []FinishedRead `json:"finished" doc:"Every read finished, newest first"`
}

// Reading gathers every read finished, the latest of each book and the earlier ones, by year.
// A share link's has the reads of the books it shows.
func (s *Service) Reading(ctx context.Context, sc *Scope) (*ReadingStats, error) {
	rows, err := s.read().ListFinishedReads(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	shown, err := s.shown(ctx, sc)
	if err != nil {
		return nil, fmt.Errorf("reading: %w", err)
	}
	out := &ReadingStats{Years: []ReadingYear{}, Finished: make([]FinishedRead, 0, len(rows))}
	for _, r := range rows {
		if shown != nil && !shown[r.BookID] {
			continue
		}
		out.Finished = append(out.Finished, FinishedRead{BookID: r.BookID, Title: r.Title, FinishedOn: r.FinishedOn, Pages: r.Pages})
		year := r.FinishedOn[:min(4, len(r.FinishedOn))]
		i := slices.IndexFunc(out.Years, func(y ReadingYear) bool { return y.Year == year })
		if i < 0 {
			out.Years = append(out.Years, ReadingYear{Year: year})
			i = len(out.Years) - 1
		}
		out.Years[i].Books++
		out.Years[i].Pages += r.Pages
	}
	slices.SortFunc(out.Years, func(a, b ReadingYear) int { return strings.Compare(b.Year, a.Year) })
	return out, nil
}
