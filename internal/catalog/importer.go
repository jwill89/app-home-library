package catalog

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// The apps an import comes from, as import refs and field sources name them.
const (
	ImportSourceBookshelf    = "bookshelf"
	ImportSourceGoodreads    = "goodreads"
	ImportSourceLibraryThing = "librarything"
	ImportSourceEndNote      = "endnote"
)

// bookshelfCut is where the Bookshelf export cuts a description off, in characters. A
// description it cut that no source has whole keeps the source "bookshelf-cut", so the cut is
// still known once the description is cleaned and no longer that long.
const (
	bookshelfCut       = 1500
	sourceBookshelfCut = "bookshelf-cut"
)

// descriptionCut reports whether a book's description is one the Bookshelf export cut off.
func descriptionCut(sources map[string]string, description string) bool {
	src := sources["description"]
	return src == sourceBookshelfCut || (src == ImportSourceBookshelf && utf8.RuneCountInString(description) == bookshelfCut)
}

// ImportOptions are the owner's choices for an import.
type ImportOptions struct {
	// NotOwned is where a book the other app does not mark as owned goes: left out ("leave"),
	// on the shelves ("library"), or on the wishlist ("wishlist"), which takes only a book with
	// an ISBN. One on a wish list or a to-read shelf goes on the wishlist whatever it says.
	NotOwned string `enum:"leave,library,wishlist"`
	// owned is the works the house has, by workKey, as the import goes on.
	owned map[string]int64
}

// ImportReport says what an import did.
type ImportReport struct {
	Source       string   `json:"source" doc:"The app the file came from: Bookshelf, Goodreads, LibraryThing, or EndNote"`
	Rows         int      `json:"rows"`
	Added        int      `json:"added" doc:"Books added"`
	Documents    int      `json:"documents" doc:"Articles, chapters, and other papers added as documents"`
	DocumentsHad int      `json:"documentsHad" doc:"Papers the library had already, found by their DOI and linked"`
	Matched      int      `json:"matched" doc:"Books the library had already, by ISBN or in another edition: linked to the other app, their reading filled in where it was blank"`
	Already      int      `json:"already" doc:"Rows imported before, with nothing to fill"`
	Filled       int      `json:"filled" doc:"Rows imported before whose book or paper had blanks the file filled; nothing it had was replaced"`
	Wanted       int      `json:"wanted" doc:"Books the other app wants, put on the wishlist"`
	NotOwned     int      `json:"notOwned" doc:"Books the other app does not mark as owned, left out (or, for the wishlist, without an ISBN)"`
	NoISBN       int      `json:"noIsbn" doc:"Rows added without an ISBN, which no lookup can fill"`
	Queued       int      `json:"queued" doc:"Editions waiting for the background lookup"`
	Problems     []string `json:"problems"`
}

// importedBook is one row of another app's export, as the library would add it.
type importedBook struct {
	Ref    string // the row's id in the other app, so the same file imported twice adds nothing
	Link   string // the book's address in the other app, for its page here
	In     BookInput
	First  CopyInput
	Copies int
	Owned  bool // the other app says the owner has it
	Wanted bool // on a wish list or a to-read shelf: put on the wishlist
}

// importFormat is an app whose export the library reads, told by the columns it has.
type importFormat struct {
	source string
	name   string
	needs  []string // lower case
	book   func(get func(string) string) *importedBook
}

// Goodreads' and LibraryThing's exports carry a "Book Id" and a "Title" as Bookshelf's do, so
// the column only each has decides; Bookshelf's is the one left.
var importFormats = []importFormat{
	{ImportSourceGoodreads, "Goodreads", []string{"book id", "title", "exclusive shelf"}, goodreadsBook},
	{ImportSourceLibraryThing, "LibraryThing", []string{"book id", "title", "primary author"}, librarythingBook},
	{ImportSourceBookshelf, "Bookshelf", []string{"book id", "isbn", "title", "authors"}, bookshelfBook},
}

// ImportFile reads an export from Bookshelf, Goodreads, or LibraryThing (CSV, or
// tab-delimited), telling which by its columns, or EndNote's XML. It adds the books the owner
// has that the library does not, links the ones it has to the other app and fills in their
// reading where it is blank, and queues each new edition for the background lookup; EndNote's
// articles and chapters become documents. The same file imported twice adds nothing the
// second time.
func (s *Service) ImportFile(ctx context.Context, r io.Reader, o ImportOptions) (*ImportReport, error) {
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	o.owned = ownedWorks(rows)
	br := bufio.NewReader(r)
	first, err := br.Peek(4096)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read the file: %w", err)
	}
	if isXML(first) {
		return s.importEndNote(ctx, br, o)
	}
	cr := csv.NewReader(br)
	cr.LazyQuotes = true
	cr.FieldsPerRecord = -1
	line, _, _ := strings.Cut(string(first), "\n")
	if strings.Count(line, "\t") > strings.Count(line, ",") {
		cr.Comma = '\t'
	}
	header, err := cr.Read()
	if err != nil {
		return nil, &FieldError{"file", "this file has no rows to read; choose the CSV or tab-delimited export itself"}
	}
	col := map[string]int{}
	for i, h := range header {
		col[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, utf8BOM)))] = i
	}
	var format *importFormat
	for i := range importFormats {
		f := &importFormats[i]
		has := true
		for _, need := range f.needs {
			if _, ok := col[need]; !ok {
				has = false
				break
			}
		}
		if has {
			format = f
			break
		}
	}
	if format == nil {
		return nil, &FieldError{"file", "this is not an export the library knows: choose the CSV export from Bookshelf, Goodreads, or LibraryThing"}
	}

	rep := &ImportReport{Source: format.name, Problems: []string{}}
	var queue []int64
	for n := 2; ; n++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n, err)
		}
		if len(rec) == 1 && strings.TrimSpace(rec[0]) == "" {
			continue // a blank line
		}
		rep.Rows++
		get := func(name string) string {
			if i, ok := col[strings.ToLower(name)]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		b := format.book(get)
		id, outcome, err := s.importBook(ctx, format, b, o)
		queue = rep.tally(fmt.Sprintf("line %d", n), b, id, outcome, err, queue)
	}
	return rep, s.lookUpImported(ctx, rep, queue)
}

// tally counts what became of one book, and adds an edition the lookup should see to queue.
func (rep *ImportReport) tally(where string, b *importedBook, id int64, outcome importOutcome, err error, queue []int64) []int64 {
	switch {
	case err != nil:
		rep.Problems = append(rep.Problems, fmt.Sprintf("%s (%s): %v", where, b.In.Title, err))
	case outcome == importAlready:
		rep.Already++
	case outcome == importFilled:
		rep.Filled++
	case outcome == importMatched:
		rep.Matched++
	case outcome == importNotOwned:
		rep.NotOwned++
	case outcome == importWanted:
		rep.Wanted++
		queue = append(queue, id)
	default:
		rep.Added++
		if b.In.ISBN == "" {
			rep.NoISBN++
		} else {
			queue = append(queue, id)
		}
	}
	return queue
}

// lookUpImported puts the editions an import added in the background lookup's queue.
func (s *Service) lookUpImported(ctx context.Context, rep *ImportReport, queue []int64) error {
	q := db.New(s.DB.Write)
	for _, id := range queue {
		if err := q.EnqueueEnrichment(ctx, db.EnqueueEnrichmentParams{EditionID: id}); err != nil {
			return err
		}
		rep.Queued++
	}
	return nil
}

type importOutcome int

const (
	importAdded importOutcome = iota
	importMatched
	importAlready
	importFilled // imported before, and its blanks filled again from the file
	importNotOwned
	importWanted
)

// importBook brings one row in and says what became of it, with the edition it added or
// matched.
func (s *Service) importBook(ctx context.Context, f *importFormat, b *importedBook, o ImportOptions) (int64, importOutcome, error) {
	if b.In.Title == "" {
		return 0, 0, errors.New("no title")
	}
	isbn13, err := b.In.normalize()
	if err != nil {
		// A bad ISBN should not cost the row: it is kept without one and said so.
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != "isbn" {
			return 0, 0, err
		}
		b.In.Notes = joinNotes(b.In.Notes, "ISBN in the "+f.name+" export did not check out: "+b.In.ISBN)
		b.In.ISBN = ""
		if isbn13, err = b.In.normalize(); err != nil {
			return 0, 0, err
		}
	}
	if b.Ref != "" {
		if copyID, err := s.read().GetImportRef(ctx, db.GetImportRefParams{Source: f.source, ExternalID: b.Ref}); err == nil {
			// Imported before: what the row says and the book lacks is filled in again, so a
			// field the other app gained since, or the library learned to read, comes across.
			c, err := s.read().GetCopy(ctx, copyID)
			if err != nil {
				return 0, 0, err
			}
			changed, _, err := s.fillImported(ctx, f.source, c.EditionID, b)
			if err != nil || !changed {
				return 0, importAlready, err
			}
			return c.EditionID, importFilled, nil
		} else if !errors.Is(err, sql.ErrNoRows) {
			return 0, 0, err
		}
	}
	if isbn13 != "" {
		if id, err := s.FindByISBN(ctx, isbn13); err != nil {
			return 0, 0, err
		} else if id > 0 {
			return id, importMatched, s.matchImported(ctx, f.source, id, b)
		}
	}
	// A book the other app does not mark as owned that the house has in another edition
	// (EndNote cites the hardcover of the owner's paperback) is the owner's: matched, not wanted.
	key := workKey(b.In.Title, firstAuthor(b.In.Contributors))
	if id := o.owned[key]; id > 0 && !b.Owned {
		return id, importMatched, s.matchImported(ctx, f.source, id, b)
	}
	// A wanted book goes on the wishlist, which keeps no import ref without a copy, so only one
	// with an ISBN, which a second import matches; one neither owned nor wanted, where asked.
	first, outcome := &b.First, importAdded
	switch {
	case b.Owned || (o.NotOwned == "library" && !b.Wanted):
	case (b.Wanted || o.NotOwned == "wishlist") && isbn13 != "":
		first, outcome = nil, importWanted
	default:
		return 0, importNotOwned, nil
	}
	var editionID int64
	err = s.inTx(ctx, func(q *db.Queries) error {
		// Asked again under the write lock: the same file imported twice at once adds once.
		if b.Ref != "" {
			if _, err := q.GetImportRef(ctx, db.GetImportRefParams{Source: f.source, ExternalID: b.Ref}); err == nil {
				outcome = importAlready
				return nil
			}
		}
		id, copyID, err := createBook(ctx, q, b.In, isbn13, first)
		if err != nil {
			return err
		}
		if first == nil {
			editionID = id
			return nil
		}
		for range b.Copies - 1 {
			if _, err := createCopy(ctx, q, id, b.First); err != nil {
				return err
			}
		}
		if b.Ref != "" {
			if err := q.CreateImportRef(ctx, db.CreateImportRefParams{Source: f.source, ExternalID: b.Ref, CopyID: copyID, Link: b.Link}); err != nil {
				return err
			}
		}
		editionID = id
		return nil
	})
	if err == nil && outcome == importAdded && key != "" && o.owned != nil && o.owned[key] == 0 {
		o.owned[key] = editionID // a later row of the same file may be another edition of it
	}
	return editionID, outcome, err
}

// matchImported links a book the library has to the row that names it in another app, and
// fills the book's blanks from it.
func (s *Service) matchImported(ctx context.Context, source string, id int64, b *importedBook) error {
	_, book, err := s.fillImported(ctx, source, id, b)
	if err != nil {
		return err
	}
	if b.Ref == "" || len(book.Copies) == 0 {
		return nil
	}
	return db.New(s.DB.Write).CreateImportRef(ctx, db.CreateImportRefParams{Source: source, ExternalID: b.Ref, CopyID: book.Copies[0].ID, Link: b.Link})
}

// fillImported fills a book's blanks from a row of another app that names it: its reading
// (status, days, rating, notes, genres), and its details as a lookup fills them, the app named
// as their source. Nothing the book has is replaced, and a field the owner emptied stays empty.
// It says whether anything changed, and gives the book as it is now.
func (s *Service) fillImported(ctx context.Context, source string, id int64, b *importedBook) (bool, *Book, error) {
	var changed bool
	var book *Book
	err := s.updateWith(ctx, id, func(had *Book, in *BookInput) error {
		book = had
		changed = fillFromRow(in, b, source)
		if !changed {
			return errUnchanged
		}
		return nil
	})
	if err != nil {
		return false, nil, err
	}
	return changed, book, nil
}

// fillFromRow fills a book's blanks from a row of another app, and says whether it changed
// anything.
func fillFromRow(in *BookInput, b *importedBook, source string) bool {
	before, _ := json.Marshal(in)
	fill := func(dst *string, v string) {
		if *dst == "" && v != "" {
			*dst = v
		}
	}
	if in.ReadStatus == "unread" && b.In.ReadStatus != "unread" {
		in.ReadStatus, in.CurrentPage = b.In.ReadStatus, b.In.CurrentPage
	}
	fill(&in.StartedOn, b.In.StartedOn)
	fill(&in.FinishedOn, b.In.FinishedOn)
	fill(&in.Notes, b.In.Notes)
	if in.Rating == 0 && b.In.Rating > 0 {
		in.Rating = b.In.Rating
	}
	if len(in.Genres) == 0 && len(b.In.Genres) > 0 {
		in.Genres = b.In.Genres
	}
	// A category worked out is a guess: one the app sorted it under is the owner's own.
	if in.Sources["category"] == sourceInferred && b.In.Category != "" {
		in.Category, in.Sources["category"] = b.In.Category, source
	}
	p := proposalOf(b.In, source)
	fillBlanks(in, &p)
	after, _ := json.Marshal(in)
	return !bytes.Equal(before, after)
}

// proposalOf is a row of another app as a lookup's answer, every field the app's, so the one
// way blanks are filled fills them from it.
func proposalOf(in BookInput, source string) lookup.Proposal {
	p := lookup.Proposal{
		Title: in.Title, Subtitle: in.Subtitle, Contributors: in.Contributors, LCCN: in.LCCN,
		Publisher: in.Publisher, PublishPlace: in.PublishPlace, Published: in.Published,
		EditionStatement: in.EditionStatement, Format: in.Format, Pages: int(in.Pages),
		HeightMM: int(in.HeightMM), WidthMM: int(in.WidthMM), ThicknessMM: int(in.ThicknessMM),
		Language: in.Language, DDC: in.DDC, LCC: in.LCC, Subjects: in.Subjects, Series: in.Series,
		SeriesNumber: in.SeriesNumber, Description: in.Description, Sources: map[string]string{},
	}
	for _, f := range lookupFields {
		p.Sources[f] = source
	}
	return p
}

// imported marks the fields a row filled as the app's, so a lookup fills only the rest. The
// row is read by its JSON names, the names sources are kept under, so every field a lookup
// can fill is marked without a second list of them.
func imported(in *BookInput, source string) {
	raw, err := json.Marshal(in)
	if err != nil {
		return
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return
	}
	for _, f := range append(slices.Clone(lookupFields), "category", "subjects") {
		switch v := values[f].(type) {
		case string:
			if v != "" {
				in.Sources[f] = source
			}
		case float64:
			if v > 0 {
				in.Sources[f] = source
			}
		case []any:
			if len(v) > 0 {
				in.Sources[f] = source
			}
		}
	}
}

// bookshelfBook reads a row of the Bookshelf app's export ("Import & Export" in its menu).
// Its other columns (quotes, flashcards, narrators, transfers) have no place in the catalog,
// and its shelves are not sections: the owner shelves without them. Every row is a book the
// owner has.
func bookshelfBook(get func(string) string) *importedBook {
	in := BookInput{
		Title:            get("Title"),
		Subtitle:         get("Subtitle"),
		Series:           get("Series"),
		SeriesNumber:     get("Volume"),
		ReadStatus:       "unread",
		StartedOn:        day(get("Started Reading On")),
		FinishedOn:       day(get("Ended Reading On")),
		Rating:           min(atoi(get("My Rating")), 5),
		Notes:            joinNotes(get("Review"), get("Short note"), get("Advanced note")),
		Genres:           splitList(get("Tags")),
		ISBN:             get("ISBN"),
		Publisher:        get("Publisher"),
		Published:        lookup.NormalizeDate(get("Published At")),
		EditionStatement: get("Edition"),
		Format:           lookup.FormatFromText(get("Format")),
		Pages:            atoi(get("Page Count")),
		Language:         lookup.NormalizeLanguage(get("Language")),
		Subjects:         splitList(get("Categories")),
		Description:      lookup.PlainText(get("Description")),
		Sources:          map[string]string{},
	}
	switch {
	case get("Read") == "1":
		in.ReadStatus = "read"
	case atoi(get("Pages Read")) > 0:
		in.ReadStatus = "reading"
		in.CurrentPage = atoi(get("Pages Read"))
	}
	for _, role := range []struct{ column, role string }{
		{"Authors", "author"}, {"Editors", "editor"}, {"Translators", "translator"},
		{"Illustrators", "illustrator"}, {"Narrators", "narrator"}, {"Photographers", "contributor"},
	} {
		for _, name := range splitNames(get(role.column)) {
			in.Contributors = append(in.Contributors, lookup.Contributor{Name: name, Role: role.role})
		}
	}
	in.Category = shelfCategory(get("Bookshelf"), &in)
	imported(&in, ImportSourceBookshelf)
	return &importedBook{
		Ref: get("Book Id"),
		In:  in,
		First: CopyInput{
			AcquiredAs:   "unknown",
			AcquiredOn:   day(get("Purchase Date")),
			AcquiredFrom: get("Purchase From"),
			PriceCents:   cents(get("Purchase Price")),
			Condition:    copyCondition(get("Condition")),
			DustJacket:   "unknown",
			Signed:       get("Signed") == "1",
		},
		Copies: max(1, int(atoi(get("Number of copies")))),
		Owned:  true,
	}
}

func atoi(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func cents(s string) int64 {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "$"))
	if s == "" {
		return -1
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return -1
	}
	return int64(f*100 + 0.5)
}

// day keeps the date part of a date or a timestamp, written with dashes or slashes.
func day(s string) string {
	if len(s) < 10 {
		return ""
	}
	d := strings.ReplaceAll(s[:10], "/", "-")
	if dayRe.MatchString(d) {
		return d
	}
	return ""
}

func splitList(s string) []string {
	return cleanList(strings.Split(s, ","))
}

// splitNames splits comma-separated names, keeping a suffix with its name ("Martin Luther
// King, Jr.").
func splitNames(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch strings.TrimSuffix(strings.ToLower(part), ".") {
		case "jr", "sr", "ii", "iii", "iv", "phd", "md":
			if len(out) > 0 {
				out[len(out)-1] += ", " + part
				continue
			}
		}
		out = append(out, part)
	}
	return out
}

func joinNotes(parts ...string) string {
	var keep []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, "\n\n")
}

// copyCondition reads a condition as the apps write it.
func copyCondition(s string) string {
	switch strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_")) {
	case "new", "as_new", "fine", "mint":
		return "fine"
	case "very_good", "like_new":
		return "very_good"
	case "good":
		return "good"
	case "fair", "acceptable":
		return "fair"
	case "poor":
		return "poor"
	}
	return "unknown"
}
