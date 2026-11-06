package catalog

import (
	"context"
	"fmt"

	"homelibrary/internal/db"
)

// GapFields are the details a book can lack that a lookup or the owner can fill, in the order
// the page lists them.
var GapFields = []string{"cover", "ddc", "heightMm", "thicknessMm", "pages", "format", "publisher", "published", "description"}

// BookGaps is a book and the details it lacks.
type BookGaps struct {
	BookID  int64    `json:"bookId"`
	Title   string   `json:"title"`
	Authors string   `json:"authors"`
	ISBN13  string   `json:"isbn13"`
	Cover   Cover    `json:"cover"`
	Missing []string `json:"missing" enum:"cover,ddc,heightMm,thicknessMm,pages,format,publisher,published,description"`
}

// GapCount is how many books lack one detail.
type GapCount struct {
	Field string `json:"field" enum:"cover,ddc,heightMm,thicknessMm,pages,format,publisher,published,description"`
	Books int64  `json:"books"`
}

// Gaps lists every book in the house that lacks a detail, with what it lacks, and how many
// books lack each.
func (s *Service) Gaps(ctx context.Context) ([]BookGaps, []GapCount, error) {
	rows, err := s.read().ListBookGaps(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("gaps: %w", err)
	}
	counts := map[string]int64{}
	out := []BookGaps{}
	for _, r := range rows {
		missing := lacks(r)
		if len(missing) == 0 {
			continue
		}
		for _, f := range missing {
			counts[f]++
		}
		out = append(out, BookGaps{
			BookID: r.ID, Title: r.Title, Authors: r.Authors, ISBN13: r.Isbn13.String,
			Cover: s.cover(r.Cover), Missing: missing,
		})
	}
	totals := make([]GapCount, 0, len(GapFields))
	for _, f := range GapFields {
		totals = append(totals, GapCount{Field: f, Books: counts[f]})
	}
	return out, totals, nil
}

func lacks(r *db.ListBookGapsRow) []string {
	has := map[string]bool{
		"cover": r.Cover != "", "ddc": r.Ddc != "", "heightMm": r.HeightMm > 0, "thicknessMm": r.ThicknessMm > 0,
		"pages": r.Pages > 0, "format": r.Format != "", "publisher": r.Publisher != "",
		"published": r.Published != "", "description": r.DescriptionLength > 0,
	}
	var out []string
	for _, f := range GapFields {
		if !has[f] {
			out = append(out, f)
		}
	}
	return out
}

// QueueLookups puts books in the background lookup's queue, to fill what they lack, each source
// asked again; one with no ISBN, which no source can find, is left out. It says how many were
// queued.
func (s *Service) QueueLookups(ctx context.Context, ids []int64) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		for _, id := range ids {
			ed, err := q.GetEdition(ctx, id)
			if err != nil || !ed.Isbn13.Valid {
				continue // gone since the page loaded, or no ISBN to look up by
			}
			if err := q.EnqueueEnrichment(ctx, db.EnqueueEnrichmentParams{EditionID: id, Fresh: 1}); err != nil {
				return fmt.Errorf("queue lookup: %w", err)
			}
			n++
		}
		return nil
	})
	return n, err
}

// QueueEveryLookup puts every book with an ISBN in the background lookup's queue, each source
// asked again, to fill whatever blanks the sources can now fill. It says how many were queued.
func (s *Service) QueueEveryLookup(ctx context.Context) (int64, error) {
	n, err := db.New(s.DB.Write).EnqueueEveryEnrichment(ctx)
	if err != nil {
		return 0, fmt.Errorf("queue every lookup: %w", err)
	}
	return n, nil
}
