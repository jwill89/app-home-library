package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// How the background lookup paces itself: a source that is down is asked again later, and
// a book no source can answer for stops being asked.
const (
	maxAttempts = 5
	idleWait    = 15 * time.Second
)

var retryAfter = []time.Duration{time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour}

// queueIfIncomplete puts an edition in the background lookup's queue when it lacks what a
// lookup could give it.
func (s *Service) queueIfIncomplete(ctx context.Context, id int64) error {
	ed, err := s.read().GetEdition(ctx, id)
	if err != nil {
		return err
	}
	if ed.Ddc != "" && ed.Cover != "" && ed.HeightMm.Valid && ed.Format != "" {
		return nil
	}
	return db.New(s.DB.Write).EnqueueEnrichment(ctx, db.EnqueueEnrichmentParams{EditionID: id})
}

// RunEnricher works through the queue until the context ends, one edition at a time.
func (s *Service) RunEnricher(ctx context.Context) {
	for {
		next, err := db.New(s.DB.Read).NextEnrichment(ctx, now())
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, sql.ErrNoRows):
			if !sleep(ctx, idleWait) {
				return
			}
			continue
		case err != nil:
			s.Logger.Error("enrichment queue", "err", err)
			if !sleep(ctx, idleWait) {
				return
			}
			continue
		}
		s.work(ctx, next.EditionID, next.Fresh == 1)
	}
}

func (s *Service) work(ctx context.Context, id int64, fresh bool) {
	q := db.New(s.DB.Write)
	err := s.fill(ctx, id, fresh)
	if ctx.Err() != nil {
		return
	}
	if err == nil || errors.Is(err, ErrNotFound) {
		if err := q.FinishEnrichment(ctx, id); err != nil {
			// Left pending and due, it would be taken again at once: a pause, so a full disk
			// or a locked database is not a loop.
			s.Logger.Error("enrichment finish", "edition", id, "err", err)
			sleep(ctx, idleWait)
		}
		return
	}
	var attempts int64
	if e, err := s.read().GetEnrichment(ctx, id); err == nil {
		attempts = e.Attempts // a miss waits the first wait
	}
	wait := retryAfter[min(int(attempts), len(retryAfter)-1)]
	s.Logger.Warn("enrichment will retry", "edition", id, "attempt", attempts+1, "err", err)
	if err := q.RetryEnrichment(ctx, db.RetryEnrichmentParams{
		MaxAttempts: maxAttempts, NextAt: time.Now().UTC().Add(wait).Format(time.RFC3339), LastError: err.Error(), EditionID: id,
	}); err != nil {
		s.Logger.Error("enrichment retry", "edition", id, "err", err)
		sleep(ctx, idleWait)
	}
}

// fill looks an edition up and fills its blanks, asking each source again when fresh and
// otherwise taking what it said last time, where it said anything. Nothing the owner typed or
// an import brought is overwritten; a plain "paperback" may become mass market or trade, which
// only says more. It fails only when no source could be asked, so the queue tries again later.
func (s *Service) fill(ctx context.Context, id int64, fresh bool) error {
	_, err := s.enrich(ctx, id, fresh)
	return err
}

// lookupFields are the details of a book a source can give, by their names in BookInput.
// Suggestions.Fields' enum is the same list, which a test holds it to.
var lookupFields = []string{"title", "subtitle", "contributors", "series", "seriesNumber", "lccn", "publisher",
	"publishPlace", "published", "editionStatement", "format", "pages", "heightMm", "widthMm", "thicknessMm",
	"language", "ddc", "lcc", "description"}

// Suggestions are what a lookup the owner asked for offers to change: the book with every
// suggested change made, and which fields those are. The book is as it was saved.
type Suggestions struct {
	Book      *Book     `json:"book"`
	Fields    []string  `json:"fields" enum:"title,subtitle,contributors,series,seriesNumber,lccn,publisher,publishPlace,published,editionStatement,format,pages,heightMm,widthMm,thicknessMm,language,ddc,lcc,description"`
	Suggested BookInput `json:"suggested"`
}

// Refresh is the owner's "look up again": every source is asked afresh, not answered from what
// it said before, the blanks are filled as Enrich fills them, and each detail the sources now
// give otherwise comes back as a suggestion to take or leave.
func (s *Service) Refresh(ctx context.Context, id int64) (*Suggestions, error) {
	res, err := s.enrich(ctx, id, true)
	if err != nil {
		return nil, err
	}
	book, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out := &Suggestions{Book: book, Fields: []string{}, Suggested: book.Input()}
	if res != nil {
		out.Fields = differences(&out.Suggested, &res.Proposal)
	}
	return out, nil
}

// enrich is Enrich, with what the sources said; nil for a book without an ISBN. fresh asks
// each source again rather than taking its answer from the cache.
func (s *Service) enrich(ctx context.Context, id int64, fresh bool) (*lookup.Result, error) {
	// The sources are asked about the book as it is now; their answers are merged into it as it
	// is when they are written, which may be seconds later.
	book, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var res *lookup.Result
	if book.ISBN13 != "" {
		res = s.Lookup.ByISBN(ctx, book.ISBN13, fresh)
		if res.Unreached() {
			return nil, fmt.Errorf("no source answered: %v", res.Failed)
		}
		if err := s.fitSeries(ctx, &res.Proposal); err != nil {
			return nil, err
		}
	}
	var related *lookup.Record
	if book.DDC == "" && (res == nil || res.Proposal.DDC == "") {
		related = s.classFromRelated(ctx, book.Title, firstAuthor(book.Contributors))
	}
	err = s.updateWith(ctx, id, func(b *Book, in *BookInput) error {
		// Markup an import brought (the Bookshelf export's encoded tags) is cleaned to the
		// text it shows; plain text is left as it is.
		in.Description = lookup.PlainText(in.Description)
		if res != nil {
			// A description the Bookshelf export cut off is not the import's whole text, so a
			// source's longer one takes its place.
			if b.DescriptionCut {
				if utf8.RuneCountInString(res.Proposal.Description) > bookshelfCut {
					in.Description = res.Proposal.Description
					in.Sources["description"] = res.Proposal.Sources["description"]
				} else {
					in.Sources["description"] = sourceBookshelfCut
				}
			}
			fillBlanks(in, &res.Proposal)
		}
		if in.DDC == "" && related != nil {
			in.DDC, in.Sources["ddc"] = related.DDC, lookup.SourceLoCRelated
			if in.LCC == "" && related.LCC != "" {
				in.LCC, in.Sources["lcc"] = related.LCC, lookup.SourceLoCRelated
			}
		}
		return s.classFromSeries(ctx, in.Series, &in.DDC, in.Sources)
	})
	if err != nil {
		return nil, fmt.Errorf("save: %w", err)
	}
	if book.Cover.ID == "" && res != nil {
		for _, c := range res.Proposal.Covers {
			err := s.coverIfNone(ctx, id, c.URL)
			if err == nil {
				break
			}
			s.Logger.Debug("cover candidate refused", "edition", id, "url", c.URL, "err", err)
		}
	}
	return res, nil
}

// classFromRelated finds the Dewey and LC numbers of another edition of the work; nil when
// the catalog has none.
func (s *Service) classFromRelated(ctx context.Context, title, author string) *lookup.Record {
	rel, err := s.Lookup.RelatedClass(ctx, title, author)
	if err != nil {
		s.Logger.Warn("related edition lookup", "title", title, "err", err)
		return nil
	}
	if rel == nil || rel.DDC == "" {
		return nil
	}
	return rel
}

// classFromSeries gives a volume no source has classified the Dewey number most of its
// series already has here, marked as the series'. A number already there stands.
func (s *Service) classFromSeries(ctx context.Context, series string, ddc *string, sources map[string]string) error {
	if *ddc != "" || series == "" {
		return nil
	}
	row, err := s.read().SeriesDDC(ctx, series)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("series class: %w", err)
	}
	*ddc, sources["ddc"] = row.Ddc, lookup.SourceSeries
	return nil
}

// fillBlanks copies into the input every field the proposal has and the input lacks. A field
// the owner emptied (its source is the owner's) is blank on purpose and stays so.
func fillBlanks(in *BookInput, p *lookup.Proposal) {
	open := func(field string) bool { return in.Sources[field] != SourceOwner }
	str := func(field string, dst *string, v string) {
		if *dst == "" && v != "" && open(field) {
			*dst = v
			in.Sources[field] = p.Sources[field]
		}
	}
	num := func(field string, dst *int64, v int) {
		if *dst == 0 && v > 0 && open(field) {
			*dst = int64(v)
			in.Sources[field] = p.Sources[field]
		}
	}
	str("subtitle", &in.Subtitle, p.Subtitle)
	str("lccn", &in.LCCN, p.LCCN)
	str("publisher", &in.Publisher, p.Publisher)
	str("publishPlace", &in.PublishPlace, p.PublishPlace)
	str("published", &in.Published, p.Published)
	str("editionStatement", &in.EditionStatement, p.EditionStatement)
	str("language", &in.Language, p.Language)
	str("ddc", &in.DDC, p.DDC)
	str("lcc", &in.LCC, p.LCC)
	str("description", &in.Description, p.Description)
	str("series", &in.Series, p.Series)
	if in.Series == p.Series {
		str("seriesNumber", &in.SeriesNumber, p.SeriesNumber)
	}
	num("pages", &in.Pages, p.Pages)
	num("heightMm", &in.HeightMM, p.HeightMM)
	num("widthMm", &in.WidthMM, p.WidthMM)
	num("thicknessMm", &in.ThicknessMM, p.ThicknessMM)
	if (in.Format == "" && open("format")) || (in.Format == "paperback" && (p.Format == "mass_market" || p.Format == "trade_paperback")) {
		if p.Format != "" {
			in.Format = p.Format
			in.Sources["format"] = p.Sources["format"]
		}
	}
	if len(in.Contributors) == 0 && len(p.Contributors) > 0 && open("contributors") {
		in.Contributors = p.Contributors
		in.Sources["contributors"] = p.Sources["contributors"]
	}
	// Subjects the owner set are the owner's list: a lookup adds none, nor puts back one taken
	// off. Otherwise the source's join the book's, up to what a book takes.
	if open("subjects") {
		in.Subjects = cleanList(append(in.Subjects, p.Subjects...))
		if len(in.Subjects) > subjectLimit {
			in.Subjects = in.Subjects[:subjectLimit]
		}
	}
}

// differences changes every field the proposal gives otherwise than the input and returns
// those fields, in the form's order. Blanks are filled before, so each one replaces something
// and is the owner's to take or leave. Spacing is no difference, and nor is a length within a
// centimetre: the Library of Congress measures in whole centimetres, rounded up.
func differences(in *BookInput, p *lookup.Proposal) []string {
	fields := []string{}
	took := func(field string) {
		in.Sources[field] = p.Sources[field]
		fields = append(fields, field)
	}
	str := func(field string, dst *string, v string) {
		if v != "" && *dst != "" && strings.Join(strings.Fields(*dst), " ") != strings.Join(strings.Fields(v), " ") {
			*dst = v
			took(field)
		}
	}
	num := func(field string, dst *int64, v int, within int64) {
		if v > 0 && *dst > 0 && max(*dst-int64(v), int64(v)-*dst) >= within {
			*dst = int64(v)
			took(field)
		}
	}
	str("title", &in.Title, p.Title)
	str("subtitle", &in.Subtitle, p.Subtitle)
	// People only when a source names someone the book lacks: one that names fewer (no
	// translator, say) knows less, not better.
	if len(in.Contributors) > 0 && slices.ContainsFunc(p.Contributors, func(c lookup.Contributor) bool { return !slices.Contains(in.Contributors, c) }) {
		in.Contributors = p.Contributors
		took("contributors")
	}
	// A series written another way is the same one. Another series brings its number, even
	// into a blank: the number the book has belongs to the series it would leave.
	if p.Series != "" && in.Series != "" && seriesKey(in.Series) != seriesKey(p.Series) {
		in.Series = p.Series
		took("series")
		if p.SeriesNumber != in.SeriesNumber {
			in.SeriesNumber = p.SeriesNumber
			took("seriesNumber")
		}
	} else {
		str("seriesNumber", &in.SeriesNumber, p.SeriesNumber)
	}
	str("lccn", &in.LCCN, p.LCCN)
	str("publisher", &in.Publisher, p.Publisher)
	str("publishPlace", &in.PublishPlace, p.PublishPlace)
	str("published", &in.Published, p.Published)
	str("editionStatement", &in.EditionStatement, p.EditionStatement)
	str("format", &in.Format, p.Format)
	num("pages", &in.Pages, p.Pages, 1)
	num("heightMm", &in.HeightMM, p.HeightMM, 10)
	num("widthMm", &in.WidthMM, p.WidthMM, 10)
	num("thicknessMm", &in.ThicknessMM, p.ThicknessMM, 10)
	str("language", &in.Language, p.Language)
	str("ddc", &in.DDC, p.DDC)
	str("lcc", &in.LCC, p.LCC)
	str("description", &in.Description, p.Description)
	return fields
}

// Input is the book as the form would send it back unchanged.
func (b *Book) Input() BookInput {
	sources := map[string]string{}
	for k, v := range b.Sources {
		sources[k] = v
	}
	return BookInput{
		Title: b.Title, Subtitle: b.Subtitle, Contributors: b.Contributors, Series: b.Series,
		SeriesNumber: b.SeriesNumber, Category: b.Category, ReadStatus: b.ReadStatus, CurrentPage: b.CurrentPage,
		StartedOn: b.StartedOn, FinishedOn: b.FinishedOn, Rating: b.Rating, Notes: b.Notes,
		Genres: b.Genres, ISBN: b.ISBN13, LCCN: b.LCCN, Publisher: b.Publisher,
		PublishPlace: b.PublishPlace, Published: b.Published, EditionStatement: b.EditionStatement,
		Format: b.Format, Pages: b.Pages, HeightMM: b.HeightMM, WidthMM: b.WidthMM,
		ThicknessMM: b.ThicknessMM, Language: b.Language, DDC: b.DDC, LCC: b.LCC,
		Subjects: b.Subjects, Description: b.Description, Sources: sources,
	}
}

// QueueCounts is how many editions are due a lookup, waiting to try again later, done with
// one, or given up on.
type QueueCounts struct {
	Pending int64 `json:"pending" doc:"Due a lookup now"`
	Waiting int64 `json:"waiting" doc:"Tried, and waiting out the pause before the next try"`
	Done    int64 `json:"done"`
	Failed  int64 `json:"failed"`
}

// Counts reports the background lookup's queue.
func (s *Service) Counts(ctx context.Context) (*QueueCounts, error) {
	rows, err := s.read().EnrichmentCounts(ctx, now())
	if err != nil {
		return nil, err
	}
	var c QueueCounts
	for _, r := range rows {
		switch r.Status {
		case "pending":
			c.Pending = r.N
		case "waiting":
			c.Waiting = r.N
		case "done":
			c.Done = r.N
		case "failed":
			c.Failed = r.N
		}
	}
	return &c, nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// today is the server's day, YYYY-MM-DD, for a record kept by the day.
func today() string {
	return time.Now().Format(time.DateOnly)
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
