package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"homelibrary/internal/covers"
	"homelibrary/internal/db"
	"homelibrary/internal/files"
	"homelibrary/internal/lookup"
)

// Service is the catalog over one database.
type Service struct {
	DB        *db.DB
	Covers    *covers.Store
	Files     *files.Store
	Lookup    *lookup.Service
	HTTP      *http.Client
	UserAgent string
	Logger    *slog.Logger
}

func (s *Service) read() *db.Queries {
	return db.New(s.DB.Read)
}

// inTx runs fn in a write transaction.
func (s *Service) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	return s.DB.InTx(ctx, fn)
}

// Create adds a book: its work, its edition, and its first copy. A cover link, when given, is
// fetched after the book is saved, and a failed fetch leaves the book without one.
func (s *Service) Create(ctx context.Context, in BookInput, first *CopyInput, coverURL string) (int64, error) {
	isbn13, err := in.normalize()
	if err != nil {
		return 0, err
	}
	if first != nil {
		if err := first.normalize(); err != nil {
			return 0, err
		}
	}
	var id int64
	err = s.inTx(ctx, func(q *db.Queries) error {
		var err error
		id, _, err = createBook(ctx, q, in, isbn13, first)
		return err
	})
	if err != nil {
		return 0, err
	}
	if coverURL != "" {
		if err := s.CoverFromURL(ctx, id, coverURL); err != nil {
			s.Logger.Warn("cover not fetched", "edition", id, "err", err)
		}
	}
	// A book saved from a lookup still lacks what only the background lookup does: another
	// edition's Dewey number, the series' number, and a cover when none was chosen.
	if isbn13 != "" {
		if err := s.queueIfIncomplete(ctx, id); err != nil {
			s.Logger.Warn("enrichment not queued", "edition", id, "err", err)
		}
	}
	return id, nil
}

// createBook writes a new work, edition, and copy with the queries given, which the import
// shares. With no first copy the book goes on the wishlist. It returns the edition and the
// copy, 0 for none.
func createBook(ctx context.Context, q *db.Queries, in BookInput, isbn13 string, first *CopyInput) (editionID, copyID int64, err error) {
	if isbn13 != "" {
		existing, err := q.GetEditionByISBN(ctx, nullString(isbn13))
		if err == nil {
			return 0, 0, &DuplicateError{ID: existing.ID}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return 0, 0, err
		}
	}
	seriesID, err := seriesOf(ctx, q, &in)
	if err != nil {
		return 0, 0, err
	}
	workID, err := q.CreateWork(ctx, db.CreateWorkParams{
		Title:        in.Title,
		SeriesID:     seriesID,
		SeriesNumber: strings.TrimSpace(in.SeriesNumber),
		ReadStatus:   in.ReadStatus,
		CurrentPage:  nullInt(in.CurrentPage),
		StartedOn:    in.StartedOn,
		FinishedOn:   in.FinishedOn,
		Rating:       nullInt(in.Rating),
		Notes:        in.Notes,
		Category:     storedCategory(&in),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("create work: %w", err)
	}
	if err := setGenres(ctx, q, workID, in.Genres); err != nil {
		return 0, 0, err
	}
	subjects, _ := json.Marshal(in.Subjects)
	sources, _ := json.Marshal(in.Sources)
	editionID, err = q.CreateEdition(ctx, db.CreateEditionParams{
		WorkID:           workID,
		Isbn13:           nullString(isbn13),
		Isbn10:           lookup.ISBN10(isbn13),
		Lccn:             strings.TrimSpace(in.LCCN),
		Title:            in.Title,
		Subtitle:         strings.TrimSpace(in.Subtitle),
		Publisher:        strings.TrimSpace(in.Publisher),
		PublishPlace:     strings.TrimSpace(in.PublishPlace),
		Published:        in.Published,
		EditionStatement: strings.TrimSpace(in.EditionStatement),
		Format:           in.Format,
		Pages:            nullInt(in.Pages),
		HeightMm:         nullInt(in.HeightMM),
		WidthMm:          nullInt(in.WidthMM),
		ThicknessMm:      nullInt(in.ThicknessMM),
		Language:         strings.TrimSpace(in.Language),
		Ddc:              in.DDC,
		Lcc:              strings.TrimSpace(in.LCC),
		Subjects:         string(subjects),
		Description:      strings.TrimSpace(in.Description),
		Sources:          string(sources),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("create edition: %w", err)
	}
	if err := setContributors(ctx, q, editionID, in.Contributors); err != nil {
		return 0, 0, err
	}
	if first == nil {
		if err := q.SetEditionWanted(ctx, db.SetEditionWantedParams{WantedOn: today(), ID: editionID}); err != nil {
			return 0, 0, fmt.Errorf("on the wishlist: %w", err)
		}
		return editionID, 0, nil
	}
	copyID, err = createCopy(ctx, q, editionID, *first)
	if err != nil {
		return 0, 0, err
	}
	return editionID, copyID, nil
}

// Update replaces a book's work and edition fields with what the owner sent.
func (s *Service) Update(ctx context.Context, id int64, in BookInput) error {
	return s.inTx(ctx, func(q *db.Queries) error { return s.update(ctx, q, id, in) })
}

// errUnchanged is what a change given to updateWith returns when it changed nothing: there is
// nothing to write.
var errUnchanged = errors.New("unchanged")

// updateWith changes a book as it is at the moment of writing, read and written in one
// transaction: a lookup that took seconds to answer merges into the book as it is now, not as
// it was when the lookup began, so nothing the owner did meanwhile is undone.
func (s *Service) updateWith(ctx context.Context, id int64, change func(b *Book, in *BookInput) error) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		b, err := s.get(ctx, q, id)
		if err != nil {
			return err
		}
		in := b.Input()
		if err := change(b, &in); errors.Is(err, errUnchanged) {
			return nil
		} else if err != nil {
			return err
		}
		return s.update(ctx, q, id, in)
	})
}

// update is Update inside a transaction already open.
func (s *Service) update(ctx context.Context, q *db.Queries, id int64, in BookInput) error {
	isbn13, err := in.normalize()
	if err != nil {
		return err
	}
	ed, err := q.GetEdition(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if isbn13 != "" && isbn13 != ed.Isbn13.String {
		if other, err := q.GetEditionByISBN(ctx, nullString(isbn13)); err == nil && other.ID != id {
			return &DuplicateError{ID: other.ID}
		}
	}
	seriesID, err := seriesOf(ctx, q, &in)
	if err != nil {
		return err
	}
	if err := q.UpdateWork(ctx, db.UpdateWorkParams{
		Title:        in.Title,
		SeriesID:     seriesID,
		SeriesNumber: strings.TrimSpace(in.SeriesNumber),
		ReadStatus:   in.ReadStatus,
		CurrentPage:  nullInt(in.CurrentPage),
		StartedOn:    in.StartedOn,
		FinishedOn:   in.FinishedOn,
		Rating:       nullInt(in.Rating),
		Notes:        in.Notes,
		Category:     storedCategory(&in),
		ID:           ed.WorkID,
	}); err != nil {
		return fmt.Errorf("update work: %w", err)
	}
	if err := setGenres(ctx, q, ed.WorkID, in.Genres); err != nil {
		return err
	}
	subjects, _ := json.Marshal(in.Subjects)
	sources, _ := json.Marshal(in.Sources)
	if err := q.UpdateEdition(ctx, db.UpdateEditionParams{
		Isbn13:           nullString(isbn13),
		Isbn10:           lookup.ISBN10(isbn13),
		Lccn:             strings.TrimSpace(in.LCCN),
		Title:            in.Title,
		Subtitle:         strings.TrimSpace(in.Subtitle),
		Publisher:        strings.TrimSpace(in.Publisher),
		PublishPlace:     strings.TrimSpace(in.PublishPlace),
		Published:        in.Published,
		EditionStatement: strings.TrimSpace(in.EditionStatement),
		Format:           in.Format,
		Pages:            nullInt(in.Pages),
		HeightMm:         nullInt(in.HeightMM),
		WidthMm:          nullInt(in.WidthMM),
		ThicknessMm:      nullInt(in.ThicknessMM),
		Language:         strings.TrimSpace(in.Language),
		Ddc:              in.DDC,
		Lcc:              strings.TrimSpace(in.LCC),
		Subjects:         string(subjects),
		Description:      strings.TrimSpace(in.Description),
		Sources:          string(sources),
		ID:               id,
	}); err != nil {
		return fmt.Errorf("update edition: %w", err)
	}
	return setContributors(ctx, q, id, in.Contributors)
}

// Delete removes an edition with its copies, its work when no other edition has it, and
// its cover files. A book with a copy out cannot be removed until the copy is back.
func (s *Service) Delete(ctx context.Context, id int64) error {
	var cover string
	err := s.inTx(ctx, func(q *db.Queries) error {
		ed, err := q.GetEdition(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := notLent(ctx, q, id, 0); err != nil {
			return err
		}
		cover = ed.Cover
		if err := q.DeleteEdition(ctx, id); err != nil {
			return err
		}
		return q.DeleteWorkWithoutEditions(ctx, ed.WorkID)
	})
	if err != nil {
		return err
	}
	if err := s.Covers.Remove(cover); err != nil {
		s.Logger.Warn("cover files left behind", "cover", cover, "err", err)
	}
	return nil
}

// Get returns a book with everything its page shows.
func (s *Service) Get(ctx context.Context, id int64) (*Book, error) {
	return s.get(ctx, s.read(), id)
}

// get is Get on the connection given: the read pool, or a write transaction's.
func (s *Service) get(ctx context.Context, q *db.Queries, id int64) (*Book, error) {
	ed, err := q.GetEdition(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	w, err := q.GetWork(ctx, ed.WorkID)
	if err != nil {
		return nil, fmt.Errorf("work: %w", err)
	}
	b := &Book{
		ID: ed.ID, WorkID: w.ID, Title: ed.Title, Subtitle: ed.Subtitle,
		Series: w.SeriesName, SeriesID: w.SeriesID.Int64, SeriesNumber: w.SeriesNumber, ReadStatus: w.ReadStatus,
		CurrentPage: w.CurrentPage.Int64, StartedOn: w.StartedOn, FinishedOn: w.FinishedOn,
		Rating: w.Rating.Int64, Notes: w.Notes,
		ISBN13: ed.Isbn13.String, ISBN10: ed.Isbn10, LCCN: ed.Lccn, Publisher: ed.Publisher,
		PublishPlace: ed.PublishPlace, Published: ed.Published, EditionStatement: ed.EditionStatement,
		Format: ed.Format, Pages: ed.Pages.Int64, HeightMM: ed.HeightMm.Int64, WidthMM: ed.WidthMm.Int64,
		ThicknessMM: ed.ThicknessMm.Int64, Language: ed.Language, DDC: ed.Ddc, LCC: ed.Lcc,
		Description: ed.Description, CreatedAt: ed.CreatedAt, UpdatedAt: ed.UpdatedAt, WantedOn: ed.WantedOn,
		Cover: s.cover(ed.Cover), Contributors: []lookup.Contributor{}, Copies: []Copy{},
	}
	_ = json.Unmarshal([]byte(ed.Subjects), &b.Subjects) // written by this package as a JSON list
	_ = json.Unmarshal([]byte(ed.Sources), &b.Sources)
	b.DescriptionCut = descriptionCut(b.Sources, b.Description)
	if b.Subjects == nil {
		b.Subjects = []string{}
	}
	if b.Sources == nil {
		b.Sources = map[string]string{}
	}
	var inferred bool
	b.Category, inferred = categoryOf(w.Category, b.DDC, b.Series, b.Title, b.Publisher, b.Subjects)
	if inferred {
		b.Sources["category"] = sourceInferred
	}
	if b.Genres, err = q.ListWorkGenres(ctx, w.ID); err != nil {
		return nil, err
	}
	people, err := q.ListEditionContributors(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, p := range people {
		b.Contributors = append(b.Contributors, lookup.Contributor{Name: p.Name, Role: p.Role})
	}
	copies, err := q.ListEditionCopies(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, c := range copies {
		b.Copies = append(b.Copies, copyOf(c))
	}
	loans, err := q.ListEditionLoansOut(ctx, id)
	if err != nil {
		return nil, err
	}
	s.withLoans(b.Copies, loans)
	if err := withLabels(ctx, q, id, b.Copies); err != nil {
		return nil, err
	}
	if b.Courses, err = bookCourses(ctx, q, id); err != nil {
		return nil, err
	}
	readings, err := q.ListWorkReadings(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	b.Readings = readingsOf(readings)
	links, err := q.ListEditionLinks(ctx, id)
	if err != nil {
		return nil, err
	}
	b.Links = make([]Link, 0, len(links))
	for _, l := range links {
		b.Links = append(b.Links, Link{Source: l.Source, URL: l.Link})
	}
	if e, err := q.GetEnrichment(ctx, id); err == nil {
		b.Enrichment = e.Status
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return b, nil
}

func (s *Service) cover(name string) Cover {
	if name == "" {
		return Cover{Widths: []int{}}
	}
	return Cover{ID: name, Widths: s.Covers.Sizes(name)}
}

// List returns every edition in the library.
func (s *Service) List(ctx context.Context) ([]BookRow, error) {
	rows, err := s.read().ListBooks(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]BookRow, 0, len(rows))
	for _, r := range rows {
		row := BookRow{
			ID: r.ID, WorkID: r.WorkID, Title: r.Title, Subtitle: r.Subtitle, Authors: r.Authors,
			Series: r.SeriesName, SeriesID: r.SeriesID, SeriesNumber: r.SeriesNumber, ISBN13: r.Isbn13.String, Format: r.Format,
			DDC: r.Ddc, Published: r.Published, Publisher: r.Publisher, ReadStatus: r.ReadStatus,
			Sections: r.Sections, Cover: s.cover(r.Cover), Copies: r.CopyCount, WantedOn: r.WantedOn, AddedAt: r.CreatedAt,
			Subjects: []string{}, Genres: r.Genres, Credits: r.Credits, LCC: r.Lcc,
		}
		_ = json.Unmarshal([]byte(r.Subjects), &row.Subjects) // written by this package as a JSON list
		row.Category, _ = categoryOf(r.Category, row.DDC, row.Series, row.Title, row.Publisher, row.Subjects)
		if r.FirstCopyID > 0 {
			row.Barcode = Barcode(r.FirstCopyID)
		}
		out = append(out, row)
	}
	// A book wanted that the house has in another edition is had already: it says which.
	owned := ownedWorks(out)
	for i := range out {
		if r := &out[i]; r.WantedOn != "" {
			r.HadAs = owned[workKey(r.Title, listedFirst(r.Authors))]
		}
	}
	return out, nil
}

// Choices are the names already in use, for the forms' suggestions.
type Choices struct {
	Sections    []string `json:"sections"`
	Genres      []string `json:"genres"`
	Series      []string `json:"series"`
	Borrowers   []string `json:"borrowers"`
	Schools     []string `json:"schools"`
	Departments []string `json:"departments"`
	Instructors []string `json:"instructors"`
}

// Choices lists the names the library already has for each thing a form names.
func (s *Service) Choices(ctx context.Context) (*Choices, error) {
	q := s.read()
	var c Choices
	for _, f := range []struct {
		into *[]string
		list func(context.Context) ([]string, error)
	}{
		{&c.Sections, q.ListSections},
		{&c.Genres, q.ListGenres},
		{&c.Series, q.ListSeriesNames},
		{&c.Borrowers, q.ListBorrowerNames},
		{&c.Schools, q.ListSchools},
		{&c.Departments, q.ListDepartments},
		{&c.Instructors, q.ListInstructors},
	} {
		names, err := f.list(ctx)
		if err != nil {
			return nil, err
		}
		*f.into = names
	}
	return &c, nil
}

// FindByISBN returns the edition with an ISBN, or 0.
func (s *Service) FindByISBN(ctx context.Context, isbn13 string) (int64, error) {
	ed, err := s.read().GetEditionByISBN(ctx, nullString(isbn13))
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return ed.ID, nil
}

// seriesOf finds or makes the book's series, by the name the library already writes it with
// (seriesKey). A series the owner put aside as none takes a book only when the owner typed
// its name, which makes it a series again; from a source or an import the book has none.
func seriesOf(ctx context.Context, q *db.Queries, in *BookInput) (sql.NullInt64, error) {
	name := strings.Join(strings.Fields(in.Series), " ")
	if name == "" {
		return sql.NullInt64{}, nil
	}
	all, err := q.ListSeries(ctx)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("series: %w", err)
	}
	if se := matchSeries(all, name); se != nil {
		if se.DismissedOn != "" {
			if in.Sources["series"] != SourceOwner {
				in.Series, in.SeriesNumber = "", ""
				return sql.NullInt64{}, nil
			}
			if err := q.SetSeriesDismissed(ctx, db.SetSeriesDismissedParams{DismissedOn: "", ID: se.ID}); err != nil {
				return sql.NullInt64{}, fmt.Errorf("series taken back: %w", err)
			}
		}
		in.Series = se.Name
		return sql.NullInt64{Int64: se.ID, Valid: true}, nil
	}
	id, err := q.UpsertSeries(ctx, name)
	if err != nil {
		return sql.NullInt64{}, fmt.Errorf("series: %w", err)
	}
	return sql.NullInt64{Int64: id, Valid: true}, nil
}

func setGenres(ctx context.Context, q *db.Queries, workID int64, genres []string) error {
	if err := q.ClearWorkGenres(ctx, workID); err != nil {
		return err
	}
	for _, g := range genres {
		id, err := q.UpsertGenre(ctx, g)
		if err != nil {
			return fmt.Errorf("genre: %w", err)
		}
		if err := q.AddWorkGenre(ctx, db.AddWorkGenreParams{WorkID: workID, GenreID: id}); err != nil {
			return err
		}
	}
	return nil
}

func setContributors(ctx context.Context, q *db.Queries, editionID int64, people []lookup.Contributor) error {
	if err := q.ClearEditionContributors(ctx, editionID); err != nil {
		return err
	}
	positions := map[string]int64{}
	seen := map[string]bool{}
	for _, c := range people {
		// The same person twice in one role is one credit.
		key := c.Role + "|" + strings.ToLower(c.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		pid, err := q.UpsertPerson(ctx, db.UpsertPersonParams{Name: c.Name, SortName: lookup.SortName(c.Name)})
		if err != nil {
			return fmt.Errorf("person: %w", err)
		}
		if err := q.AddEditionContributor(ctx, db.AddEditionContributorParams{EditionID: editionID, PersonID: pid, Role: c.Role, Position: positions[c.Role]}); err != nil {
			return err
		}
		positions[c.Role]++
	}
	return nil
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt(n int64) sql.NullInt64 {
	if n <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}
