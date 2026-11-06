package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// RetailCodeMessage is what the owner is told on scanning a store's product code.
const RetailCodeMessage = "That barcode is a store's product code, not the ISBN. On a mass-market paperback the ISBN is printed inside the front cover or on the back near the price: scan that one, or type it."

// A scan is tried this many times when no source can be reached, then left as failed.
const scanAttempts = 3

// ScanInput is a barcode scanned on the Scan page.
type ScanInput struct {
	Code string `json:"code" minLength:"7" maxLength:"20" doc:"An ISBN as scanned or typed; one of the library's own labels is refused, its book being here already"`
}

// AddScansInput is the owner's word to add found books, with what their copies share.
type AddScansInput struct {
	IDs        []int64 `json:"ids" minItems:"1" maxItems:"300" doc:"The found scans to add"`
	Section    string  `json:"section" maxLength:"100"`
	AcquiredAs string  `json:"acquiredAs" enum:"unknown,new,used"`
	Wanted     bool    `json:"wanted" doc:"Put them on the wishlist instead of the shelves"`
}

// Scan is one entry in the list of scans.
type Scan struct {
	ID        int64  `json:"id"`
	ISBN13    string `json:"isbn13"`
	Section   string `json:"section"`
	Status    string `json:"status" enum:"pending,found,adding,added,owned,not_found,failed" doc:"Being looked up, found and waiting for the owner, on its way in, added, already in the library, unknown to every source, or not reached"`
	BookID    int64  `json:"bookId" doc:"The book added or already owned; 0 for none"`
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Cover     Cover  `json:"cover" doc:"The cover of the book added or owned"`
	CoverURL  string `json:"coverUrl" doc:"A found book's cover at its source, until it is added"`
	WantedOn  string `json:"wantedOn" doc:"The day its book went on the wishlist; empty when it is not wanted"`
	Error     string `json:"error"`
	ScannedAt string `json:"scannedAt"`
}

func (s *Service) scanOf(r *db.ScanRow) Scan {
	out := Scan{
		ID: r.ID, ISBN13: r.Isbn13, Section: r.Section, Status: r.Status, BookID: r.EditionID,
		Title: r.Title, Authors: r.Authors, Cover: s.cover(r.Cover), Error: r.Error, ScannedAt: r.ScannedAt,
		WantedOn: r.WantedOn,
	}
	// A book not yet in the library is shown as the sources describe it.
	var p lookup.Proposal
	if r.EditionID == 0 && r.Proposal != "" && json.Unmarshal([]byte(r.Proposal), &p) == nil {
		out.Title = p.Title
		var authors []string
		for _, c := range p.Contributors {
			if c.Role == "author" {
				authors = append(authors, c.Name)
			}
		}
		out.Authors = strings.Join(authors, ", ")
		if len(p.Covers) > 0 {
			out.CoverURL = p.Covers[0].URL
		}
	}
	return out
}

// isbnOf reads a typed or scanned code as an ISBN-13, or says what else it is: one of the
// library's own labels, a store's product code, or nothing at all. field names the input.
func isbnOf(field, code string) (string, error) {
	if isbn, ok := lookup.NormalizeISBN(code); ok {
		return isbn, nil
	}
	if copyID, own := ParseBarcode(code); own {
		return "", &FieldError{field, fmt.Sprintf("That is your own label, %s: the book is already here.", Barcode(copyID))}
	}
	if lookup.IsRetailCode(code) {
		return "", &FieldError{field, RetailCodeMessage}
	}
	return "", &FieldError{field, "That is not an ISBN. Check the digits, or type them."}
}

// ISBNLookup is what the sources say of an ISBN, and the book the library has with it.
type ISBNLookup struct {
	lookup.Result
	ExistingID int64 `json:"existingId" doc:"The book the library already has with this ISBN; 0 for none"`
}

// LookUpISBN asks every source about a typed or scanned ISBN, for the owner to review before
// adding it. A volume no source has classified takes the number its series has here.
func (s *Service) LookUpISBN(ctx context.Context, code string, fresh bool) (*ISBNLookup, error) {
	isbn, err := isbnOf("isbn", code)
	if err != nil {
		return nil, err
	}
	existing, err := s.FindByISBN(ctx, isbn)
	if err != nil {
		return nil, err
	}
	out := &ISBNLookup{Result: *s.Lookup.ByISBN(ctx, isbn, fresh), ExistingID: existing}
	p := &out.Proposal
	if err := s.fitSeries(ctx, p); err != nil {
		return nil, err
	}
	if err := s.classFromSeries(ctx, p.Series, &p.DDC, p.Sources); err != nil {
		return nil, err
	}
	return out, nil
}

// QueueScan records a scanned ISBN for the background to look up and hold for the owner. A
// book already in the library is reported at once and nothing is looked up; the same book
// scanned twice while it is on the list is the same scan.
func (s *Service) QueueScan(ctx context.Context, in ScanInput) (*Scan, error) {
	isbn, err := isbnOf("code", in.Code)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.inTx(ctx, func(q *db.Queries) error {
		open, err := q.OpenScanForISBN(ctx, isbn)
		if err == nil {
			id = open
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		p := db.CreateScanParams{Isbn13: isbn, Status: "pending"}
		ed, err := q.GetEditionByISBN(ctx, nullString(isbn))
		switch {
		case err == nil:
			p.Status = "owned"
			p.EditionID = sql.NullInt64{Int64: ed.ID, Valid: true}
			p.DoneAt = sql.NullString{String: now(), Valid: true}
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
		id, err = q.CreateScan(ctx, p)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetScan(ctx, id)
}

// GetScan returns one scan as the review list shows it.
func (s *Service) GetScan(ctx context.Context, id int64) (*Scan, error) {
	r, err := s.read().GetScanView(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	out := s.scanOf(r)
	return &out, nil
}

// OpenScans lists the scans not yet marked done, newest first, and how many the background is
// still working on.
func (s *Service) OpenScans(ctx context.Context) ([]Scan, int64, error) {
	q := s.read()
	rows, err := q.ListOpenScans(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Scan, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.scanOf(r))
	}
	pending, err := q.CountPendingScans(ctx)
	return out, pending, err
}

// AddScans marks found scans to be added, with the copy details the owner chose, and says how
// many will be. The background adds them in turn, as it looks scans up.
func (s *Service) AddScans(ctx context.Context, in AddScansInput) (int64, error) {
	var n int64
	err := s.inTx(ctx, func(q *db.Queries) error {
		for _, id := range in.IDs {
			k, err := q.AddFoundScan(ctx, db.AddFoundScanParams{Section: strings.TrimSpace(in.Section), AcquiredAs: storedChoice(in.AcquiredAs), Want: boolInt(in.Wanted), ID: id})
			if err != nil {
				return fmt.Errorf("add scan %d: %w", id, err)
			}
			n += k
		}
		return nil
	})
	return n, err
}

// ReviewScan takes a settled scan off the list; a book it added stays, and a found book not
// added is let go.
func (s *Service) ReviewScan(ctx context.Context, id int64) error {
	n, err := db.New(s.DB.Write).ReviewScan(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReviewFinishedScans takes every settled scan off the list.
func (s *Service) ReviewFinishedScans(ctx context.Context) error {
	return db.New(s.DB.Write).ReviewFinishedScans(ctx)
}

// UndoScan removes a scan made by mistake, and with removeBook the book it added: a scan of
// the wrong barcode should leave nothing behind.
func (s *Service) UndoScan(ctx context.Context, id int64, removeBook bool) error {
	sc, err := s.read().GetScan(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if removeBook && sc.Status == "added" && sc.EditionID.Valid {
		if err := s.Delete(ctx, sc.EditionID.Int64); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return db.New(s.DB.Write).DeleteScan(ctx, id)
}

// RunScans looks up the queued scans and adds the found books the owner said to add, oldest
// first, until the context ends.
func (s *Service) RunScans(ctx context.Context) {
	for {
		sc, err := db.New(s.DB.Read).NextScanWork(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, sql.ErrNoRows):
			if !sleep(ctx, 2*time.Second) {
				return
			}
			continue
		case err != nil:
			s.Logger.Error("scan queue", "err", err)
			if !sleep(ctx, idleWait) {
				return
			}
			continue
		}
		s.workScan(ctx, sc)
	}
}

func (s *Service) workScan(ctx context.Context, sc *db.Scan) {
	work := s.lookUpScan
	if sc.Status == "adding" {
		work = s.addFoundScan
	}
	err := work(ctx, sc)
	if err == nil || ctx.Err() != nil {
		return
	}
	s.Logger.Warn("scan will retry", "scan", sc.ID, "status", sc.Status, "attempt", sc.Attempts+1, "err", err)
	if err := db.New(s.DB.Write).RetryScan(ctx, db.RetryScanParams{MaxAttempts: scanAttempts, Error: err.Error(), ID: sc.ID}); err != nil {
		s.Logger.Error("scan retry", "scan", sc.ID, "err", err)
	}
	sleep(ctx, 5*time.Second)
}

// lookUpScan asks the sources about a scanned ISBN and holds what they said for the owner. It
// fails only when no source could be asked.
func (s *Service) lookUpScan(ctx context.Context, sc *db.Scan) error {
	q := db.New(s.DB.Write)
	settle := func(status string, edition int64) error {
		return q.FinishScan(ctx, db.FinishScanParams{Status: status, EditionID: sql.NullInt64{Int64: edition, Valid: edition > 0}, ID: sc.ID})
	}
	if id, err := s.FindByISBN(ctx, sc.Isbn13); err != nil {
		return err
	} else if id > 0 {
		return settle("owned", id)
	}
	res := s.Lookup.ByISBN(ctx, sc.Isbn13, false)
	if res.Unreached() {
		return fmt.Errorf("no source answered: %v", res.Failed)
	}
	if !res.Found() || res.Proposal.Title == "" {
		return settle("not_found", 0)
	}
	if err := s.fitSeries(ctx, &res.Proposal); err != nil {
		return err
	}
	proposal, err := json.Marshal(res.Proposal)
	if err != nil {
		return fmt.Errorf("keep scan %d lookup: %w", sc.ID, err)
	}
	return q.FindScan(ctx, db.FindScanParams{Proposal: string(proposal), ID: sc.ID})
}

// addFoundScan adds a found book the owner said to add, from what the sources said of it at
// the scan, with the copy details the owner chose.
func (s *Service) addFoundScan(ctx context.Context, sc *db.Scan) error {
	var p lookup.Proposal
	if err := json.Unmarshal([]byte(sc.Proposal), &p); err != nil {
		return fmt.Errorf("read scan %d lookup: %w", sc.ID, err)
	}
	in := InputFromProposal(&p)
	if err := s.classFromSeries(ctx, in.Series, &in.DDC, in.Sources); err != nil {
		return err
	}
	cover := ""
	if len(p.Covers) > 0 {
		cover = p.Covers[0].URL
	}
	first := &CopyInput{Section: sc.Section, AcquiredAs: shownChoice(sc.AcquiredAs), PriceCents: -1, Condition: "unknown", DustJacket: "unknown"}
	if sc.Want == 1 {
		first = nil // on the wishlist
	}
	status := "added"
	id, err := s.Create(ctx, in, first, cover)
	// Added meanwhile, by hand or by another scan: the book is the library's already.
	var dup *DuplicateError
	if errors.As(err, &dup) {
		status, id, err = "owned", dup.ID, nil
	}
	if err != nil {
		return err
	}
	return db.New(s.DB.Write).FinishScan(ctx, db.FinishScanParams{Status: status, EditionID: sql.NullInt64{Int64: id, Valid: true}, ID: sc.ID})
}

// InputFromProposal is a merged lookup as a book to add, every field marked with its source.
func InputFromProposal(p *lookup.Proposal) BookInput {
	sources := map[string]string{}
	for k, v := range p.Sources {
		sources[k] = v
	}
	toInt := func(n int) int64 { return int64(n) }
	return BookInput{
		Title: p.Title, Subtitle: p.Subtitle, Contributors: p.Contributors, Series: p.Series,
		SeriesNumber: p.SeriesNumber, ReadStatus: "unread", ISBN: p.ISBN13, LCCN: p.LCCN,
		Publisher: p.Publisher, PublishPlace: p.PublishPlace, Published: p.Published,
		EditionStatement: p.EditionStatement, Format: p.Format, Pages: toInt(p.Pages),
		HeightMM: toInt(p.HeightMM), WidthMM: toInt(p.WidthMM), ThicknessMM: toInt(p.ThicknessMM),
		Language: p.Language, DDC: p.DDC, LCC: p.LCC, Subjects: p.Subjects, Description: p.Description,
		Sources: sources,
	}
}

// Identified is what a scanned barcode turned out to be.
type Identified struct {
	Kind    string `json:"kind" enum:"isbn,copy,retail" doc:"An ISBN, one of the library's own labels, or a store's product code"`
	ISBN13  string `json:"isbn13"`
	CopyID  int64  `json:"copyId" doc:"The copy a label is on; 0 for a code that is not a label"`
	Barcode string `json:"barcode" doc:"That copy's label, HL and its number"`
	BookID  int64  `json:"bookId" doc:"The book in the library with this code; 0 for none"`
	Title   string `json:"title"`
	Authors string `json:"authors"`
	Cover   Cover  `json:"cover"`
	Copies  int    `json:"copies"`
	Message string `json:"message" doc:"What to tell the owner, for a code that is not an ISBN"`
}

// Identify says what a scanned code is and whether the library has its book: the "do I own
// this?" of a bookstore. It changes nothing.
func (s *Service) Identify(ctx context.Context, code string) (*Identified, error) {
	out := &Identified{}
	var id int64
	if copyID, ok := ParseBarcode(code); ok {
		out.Kind, out.CopyID, out.Barcode = "copy", copyID, Barcode(copyID)
		var err error
		id, err = s.EditionOfCopy(ctx, copyID)
		if errors.Is(err, ErrNotFound) {
			out.Message = fmt.Sprintf("%s is one of your labels, but that copy is no longer in the library.", Barcode(copyID))
			return out, nil
		}
		if err != nil {
			return nil, err
		}
	} else if isbn, ok := lookup.NormalizeISBN(code); ok {
		out.Kind, out.ISBN13 = "isbn", isbn
		var err error
		if id, err = s.FindByISBN(ctx, isbn); err != nil {
			return nil, err
		}
	} else if lookup.IsRetailCode(code) {
		out.Kind, out.Message = "retail", RetailCodeMessage
		return out, nil
	} else {
		return nil, &FieldError{"code", "That is not an ISBN or one of the library's labels."}
	}
	if id == 0 {
		return out, nil
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	out.BookID, out.Title, out.Cover, out.Copies, out.ISBN13 = b.ID, b.Title, b.Cover, len(b.Copies), b.ISBN13
	var authors []string
	for _, c := range b.Contributors {
		if c.Role == "author" {
			authors = append(authors, c.Name)
		}
	}
	out.Authors = strings.Join(authors, ", ")
	return out, nil
}
