package catalog

import (
	"cmp"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Export is the whole library as one JSON document, everything the owner entered and every
// source's part in it, to keep elsewhere or to take to another app. Covers and document files
// are not in it; the backups keep those.
type Export struct {
	App        string         `json:"app"`
	ExportedAt string         `json:"exportedAt"`
	Books      []Book         `json:"books"`
	Borrowers  []Borrower     `json:"borrowers"`
	Loans      []Loan         `json:"loans"`
	Courses    []CourseDetail `json:"courses"`
	Documents  []Document     `json:"documents"`
	Bookcases  []Bookcase     `json:"bookcases"`
	Areas      []Area         `json:"areas"`
	Shelving   []ShelvingRule `json:"shelving"`
}

// Export gathers the whole library.
func (s *Service) Export(ctx context.Context) (*Export, error) {
	// Every list is a list, empty or not, for whatever reads the file.
	out := &Export{
		App: "Home Library", ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Books: []Book{}, Courses: []CourseDetail{}, Documents: []Document{},
	}
	defer func() {
		if out.Areas == nil {
			out.Areas = []Area{}
		}
		if out.Bookcases == nil {
			out.Bookcases = []Bookcase{}
		}
		if out.Borrowers == nil {
			out.Borrowers = []Borrower{}
		}
		if out.Loans == nil {
			out.Loans = []Loan{}
		}
		if out.Shelving == nil {
			out.Shelving = []ShelvingRule{}
		}
	}()
	rows, err := s.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("export books: %w", err)
	}
	for _, r := range rows {
		b, err := s.Get(ctx, r.ID)
		if errors.Is(err, ErrNotFound) {
			continue // removed while the export ran
		}
		if err != nil {
			return nil, fmt.Errorf("export book %d: %w", r.ID, err)
		}
		out.Books = append(out.Books, *b)
	}
	if out.Borrowers, err = s.Borrowers(ctx); err != nil {
		return nil, fmt.Errorf("export borrowers: %w", err)
	}
	if out.Loans, err = s.Loans(ctx); err != nil {
		return nil, fmt.Errorf("export loans: %w", err)
	}
	courses, err := s.Courses(ctx)
	if err != nil {
		return nil, fmt.Errorf("export courses: %w", err)
	}
	for _, c := range courses {
		d, err := s.GetCourse(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("export course %d: %w", c.ID, err)
		}
		out.Courses = append(out.Courses, *d)
	}
	docs, err := s.Documents(ctx, "", nil)
	if err != nil {
		return nil, fmt.Errorf("export documents: %w", err)
	}
	for _, r := range docs {
		d, err := s.GetDocument(ctx, r.ID)
		if err != nil {
			return nil, fmt.Errorf("export document %d: %w", r.ID, err)
		}
		out.Documents = append(out.Documents, *d)
	}
	if out.Areas, err = s.Areas(ctx); err != nil {
		return nil, fmt.Errorf("export areas: %w", err)
	}
	if out.Bookcases, err = s.Bookcases(ctx); err != nil {
		return nil, fmt.Errorf("export bookcases: %w", err)
	}
	if out.Shelving, err = s.ShelvingRules(ctx); err != nil {
		return nil, fmt.Errorf("export shelving: %w", err)
	}
	return out, nil
}

// inert keeps a spreadsheet from running a cell as a formula: text a source gave could begin
// "=HYPERLINK(" and send the sheet's contents away when it opens.
func inert(cell string) string {
	if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
		return "'" + cell
	}
	return cell
}

// csvColumns are the books export's columns, one row a copy. Lengths are millimetres, money
// is dollars, dates are YYYY-MM-DD, and lists are joined with "; ".
var csvColumns = []string{
	"Barcode", "Title", "Subtitle", "Authors", "Other contributors", "Series", "Series number",
	"Category", "ISBN-13", "ISBN-10", "LCCN", "Publisher", "Place", "Published", "Edition", "Format",
	"Pages", "Height (mm)", "Width (mm)", "Thickness (mm)", "Language", "Dewey", "LC class",
	"Call number", "Subjects", "Genres", "Description", "Read status", "Current page",
	"Started", "Finished", "Earlier reads", "Rating", "Book notes", "Section", "Bought", "Acquired on",
	"Acquired from", "Price", "Condition", "Dust jacket", "Signed", "Location", "Copy notes",
	"Lent to", "Lent on", "Due back", "Gone", "Gone on", "Gone note", "Last seen", "Added",
	"Wanted since",
}

const utf8BOM = "\xef\xbb\xbf"

// WriteBooksCSV writes the books as a spreadsheet reads them: a row for each copy, and one for
// a book with none. The byte order mark tells Excel the text is UTF-8.
func WriteBooksCSV(w io.Writer, books []Book) error {
	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(csvColumns); err != nil {
		return err
	}
	num := func(n int64) string {
		if n <= 0 {
			return ""
		}
		return strconv.FormatInt(n, 10)
	}
	for i := range books {
		b := &books[i]
		var authors, others []string
		for _, c := range b.Contributors {
			if c.Role == "author" {
				authors = append(authors, c.Name)
			} else {
				others = append(others, c.Name+" ("+c.Role+")")
			}
		}
		book := []string{
			b.Title, b.Subtitle, strings.Join(authors, "; "), strings.Join(others, "; "), b.Series,
			b.SeriesNumber, b.Category, b.ISBN13, b.ISBN10, b.LCCN, b.Publisher, b.PublishPlace, b.Published,
			b.EditionStatement, b.Format, num(b.Pages), num(b.HeightMM), num(b.WidthMM),
			num(b.ThicknessMM), b.Language, b.DDC, b.LCC,
		}
		reading := []string{
			strings.Join(b.Subjects, "; "), strings.Join(b.Genres, "; "), b.Description,
			b.ReadStatus, num(b.CurrentPage), b.StartedOn, b.FinishedOn, earlier(b.Readings), num(b.Rating), b.Notes,
		}
		copies := b.Copies
		if len(copies) == 0 {
			copies = []Copy{{}}
		}
		for _, c := range copies {
			row := append([]string{c.Barcode}, book...)
			row = append(row, strings.Join(c.CallNumber, " "))
			row = append(row, reading...)
			row = append(row, copyColumns(&c)...)
			row = append(row, b.CreatedAt, b.WantedOn)
			for i := range row {
				row[i] = inert(row[i])
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}

// earlier is a book's earlier reads in one cell: the day each finished, or began.
func earlier(rs []Reading) string {
	days := make([]string, 0, len(rs))
	for _, r := range rs {
		days = append(days, cmp.Or(r.FinishedOn, r.StartedOn))
	}
	return strings.Join(days, "; ")
}

// copyColumns are a copy's own columns, empty for a book with no copy.
func copyColumns(c *Copy) []string {
	if c.ID == 0 {
		return make([]string, 17)
	}
	price := ""
	if c.PriceCents >= 0 {
		price = fmt.Sprintf("%d.%02d", c.PriceCents/100, c.PriceCents%100)
	}
	known := func(v string) string {
		if v == "unknown" {
			return ""
		}
		return v
	}
	signed := ""
	if c.Signed {
		signed = "yes"
	}
	lentTo, lentOn, due := "", "", ""
	if c.Loan != nil {
		lentTo, lentOn, due = c.Loan.Borrower, c.Loan.LentOn, c.Loan.DueOn
	}
	return []string{
		c.Section, known(c.AcquiredAs), c.AcquiredOn, c.AcquiredFrom, price, known(c.Condition),
		known(c.DustJacket), signed, c.Location, c.Notes, lentTo, lentOn, due,
		c.GoneAs, c.GoneOn, c.GoneNote, c.SeenOn,
	}
}
