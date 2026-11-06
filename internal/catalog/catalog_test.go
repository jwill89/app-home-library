package catalog

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"homelibrary/internal/covers"
	"homelibrary/internal/db"
	"homelibrary/internal/files"
	"homelibrary/internal/lookup"
)

// local answers every request from the fixtures, whatever host it was for, so nothing
// leaves the machine.
type local struct{ h http.Handler }

func (l local) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	l.h.ServeHTTP(w, r)
	return w.Result(), nil
}

func fixtureFile(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "lookup", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func coverPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 300, 450))
	for i := range img.Pix {
		img.Pix[i] = 200
	}
	img.Set(1, 1, color.Black)
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func newService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := db.Migrate(ctx, database.Write); err != nil {
		t.Fatal(err)
	}
	store, err := covers.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	docs, err := files.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	png := coverPNG(t)
	mux := http.NewServeMux()
	mux.HandleFunc("lx2.loc.gov/lcdb", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		switch {
		case strings.Contains(q, "bath.isbn=9780425268285"):
			_, _ = w.Write(fixtureFile(t, "loc-9780425268285.xml"))
		case strings.Contains(q, "dc.title"):
			_, _ = w.Write(fixtureFile(t, "loc-search-blood-song.xml"))
		default:
			_, _ = w.Write(fixtureFile(t, "loc-9780425268285.xml"))
		}
	})
	mux.HandleFunc("openlibrary.org/api/books", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Query().Get("bibkeys"), "9780425268285") {
			_, _ = w.Write([]byte("{}"))
			return
		}
		_, _ = w.Write(fixtureFile(t, "ol-"+r.URL.Query().Get("jscmd")+"-9780425268285.json"))
	})
	mux.HandleFunc("covers.openlibrary.org/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	// OpenAlex knows an open copy of 10.1000/open, a PDF, and of 10.1000/page, a web page.
	mux.HandleFunc("api.openalex.org/works/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "10.1000/open"):
			_, _ = w.Write([]byte(`{"best_oa_location":{"pdf_url":"https://oa.example.org/paper.pdf","landing_page_url":"https://oa.example.org/paper","license":"cc-by","version":"acceptedVersion","source":{"display_name":"A Repository"}}}`))
		case strings.HasSuffix(r.URL.Path, "10.1000/page"):
			_, _ = w.Write([]byte(`{"best_oa_location":{"pdf_url":"https://oa.example.org/about","landing_page_url":"https://oa.example.org/about"}}`))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("oa.example.org/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/paper.pdf" {
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4\n%open copy\n"))
			return
		}
		_, _ = w.Write([]byte("<!doctype html><title>About the paper</title>"))
	})
	client := &http.Client{Transport: local{mux}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Service{
		DB:     database,
		Covers: store,
		Files:  docs,
		Lookup: lookup.New(lookup.Options{UserAgent: "test", HTTPClient: client, Logger: logger, Cache: &LookupCache{DB: database, Logger: logger}}),
		HTTP:   client,
		Logger: logger,
	}
}

const bookshelfCSV = `"Book Id","ISBN","Bookshelf","Tags","Wishlist","Started Reading On","Ended Reading On","Pages Read","Read","My Rating","Review","Short note","Advanced note","Signed","Condition","Number of copies","Quotes","Flashcards","Title","Subtitle","Language","Categories","Authors","Illustrators","Translators","Editors","Narrators","Photographers","Publisher","Page Count","Published At","Format","Edition","Series","Volume","Loan Type","Loan Name","Loan Start Date","Loan Due Date","Purchase Type","Purchase From","Purchase Price","Purchase Date","Transfer Type","Transfer place","Transfer Price","Transfer Date","Description","Date added"
"1","9780425268285","Fiction","",0,,,0,1,4,,,,0,,1,"","","Blood Song",,"en","Fantasy,Epic","Anthony Ryan","","","","","","Ace",624,"2014-07-01","paperback",,"Raven's Shadow",1,,,,,"purchased","Half Price Books","7.99","2020-05-01",,,,,"A boy is given to the Sixth Order.","2026-03-05"
"2","","Nonfiction","",0,,,120,0,,,,,0,,2,"","","Course Reader",,"English","Philosophy","Martin Luther King, Jr.","","","","","","Campus Press",300,"2001-01-01","hardcover",,,,,,,,,,,,,,,,,"2026-03-05"
`

func TestImportBookshelfAndEnrich(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	rep, err := s.ImportFile(ctx, strings.NewReader(bookshelfCSV), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Rows != 2 || rep.Added != 2 || rep.NoISBN != 1 || rep.Queued != 1 || len(rep.Problems) != 0 {
		t.Fatalf("report %+v", rep)
	}
	again, err := s.ImportFile(ctx, strings.NewReader(bookshelfCSV), ImportOptions{})
	if err != nil || again.Added != 0 || again.Already != 2 {
		t.Fatalf("second import %+v, err %v", again, err)
	}

	rows, err := s.List(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %d, err %v", len(rows), err)
	}
	blood, reader := rows[0], rows[1]
	if blood.Authors != "Anthony Ryan" || blood.Sections != "" || blood.Series != "Raven's Shadow" || blood.ReadStatus != "read" || blood.Barcode != "HL00001" {
		t.Errorf("first row %+v", blood)
	}
	if reader.Authors != "Martin Luther King, Jr." || reader.Copies != 2 || reader.ReadStatus != "reading" {
		t.Errorf("second row %+v", reader)
	}
	book, err := s.Get(ctx, blood.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c := book.Copies[0]; c.AcquiredFrom != "Half Price Books" || c.PriceCents != 799 || c.AcquiredOn != "2020-05-01" {
		t.Errorf("copy %+v", c)
	}

	// The background lookup fills what the export lacked: Open Library knows it is a mass
	// market paperback, and the catalog's hardcover gives the Dewey number.
	if err := s.fill(ctx, blood.ID, false); err != nil {
		t.Fatal(err)
	}
	book, err = s.Get(ctx, blood.ID)
	if err != nil {
		t.Fatal(err)
	}
	if book.Format != "mass_market" || book.DDC != "823.92" || book.Sources["ddc"] != lookup.SourceLoCRelated {
		t.Errorf("format %q, ddc %q from %q", book.Format, book.DDC, book.Sources["ddc"])
	}
	if book.LCC != "PR6118.Y3523 B56 2014" || book.Description != "A boy is given to the Sixth Order." || book.Pages != 624 {
		t.Errorf("lcc %q, description %q, pages %d", book.LCC, book.Description, book.Pages)
	}
	// The test cover is 300 pixels wide, so only the 200 rendition is smaller than it.
	if book.Cover.ID == "" || len(book.Cover.Widths) != 1 || book.Sources["cover"] != lookup.SourceOpenLibrary {
		t.Errorf("cover %+v from %q", book.Cover, book.Sources["cover"])
	}
}

func TestCreateRefusesADuplicateISBN(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	in := BookInput{Title: "Blood Song", ISBN: "0425268284", ReadStatus: "unread", Sources: map[string]string{}}
	id, err := s.Create(ctx, in, &CopyInput{Section: "Fiction", AcquiredAs: "used", Condition: "good", DustJacket: "unknown", PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, in, &CopyInput{}, "")
	var dup *DuplicateError
	if !errors.As(err, &dup) || dup.ID != id {
		t.Errorf("second create: %v", err)
	}
	if _, err := s.Create(ctx, BookInput{Title: "x", ISBN: "1234567890"}, &CopyInput{}, ""); err == nil {
		t.Error("a bad ISBN was taken")
	}
	book, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if book.ISBN13 != "9780425268285" || book.Copies[0].AcquiredAs != "used" || book.Copies[0].DustJacket != "unknown" {
		t.Errorf("book %+v", book)
	}
	if id2, ok := ParseBarcode(book.Copies[0].Barcode); !ok || id2 != book.Copies[0].ID {
		t.Errorf("barcode %q", book.Copies[0].Barcode)
	}
}

func TestScanQueue(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	scan := ScanInput{Code: "0425268284"}

	first, err := s.QueueScan(ctx, scan)
	if err != nil || first.Status != "pending" || first.ISBN13 != "9780425268285" {
		t.Fatalf("first scan %+v, err %v", first, err)
	}
	again, err := s.QueueScan(ctx, scan)
	if err != nil || again.ID != first.ID {
		t.Errorf("a second scan of the same book made another: %+v, err %v", again, err)
	}

	work := func() {
		t.Helper()
		sc, err := db.New(s.DB.Read).NextScanWork(ctx)
		if err != nil {
			t.Fatal(err)
		}
		s.workScan(ctx, sc)
	}

	// Looked up, the book waits for the owner, shown as the sources describe it.
	work()
	found, err := s.GetScan(ctx, first.ID)
	if err != nil || found.Status != "found" || found.Title != "Blood Song" || found.BookID != 0 || found.Authors != "Anthony Ryan" {
		t.Fatalf("after the lookup %+v, err %v", found, err)
	}
	if again, _ := s.QueueScan(ctx, scan); again.ID != first.ID {
		t.Errorf("a found book scanned again made another scan: %+v", again)
	}

	n, err := s.AddScans(ctx, AddScansInput{IDs: []int64{first.ID}, Section: "Fiction", AcquiredAs: "used"})
	if err != nil || n != 1 {
		t.Fatalf("add %d, err %v", n, err)
	}
	if n, _ := s.AddScans(ctx, AddScansInput{IDs: []int64{first.ID}}); n != 0 {
		t.Errorf("a scan being added was marked to add again")
	}
	if _, pending, _ := s.OpenScans(ctx); pending != 1 {
		t.Errorf("a scan being added is not counted as work: %d", pending)
	}
	work()
	added, err := s.GetScan(ctx, first.ID)
	if err != nil || added.Status != "added" || added.Title != "Blood Song" || added.BookID == 0 {
		t.Fatalf("after adding %+v, err %v", added, err)
	}
	book, err := s.Get(ctx, added.BookID)
	if err != nil || book.Copies[0].Section != "Fiction" || book.Copies[0].AcquiredAs != "used" {
		t.Errorf("the added book's copy %+v, err %v", book.Copies, err)
	}

	// Once reviewed, scanning it again says it is already here, without looking anything up.
	if err := s.ReviewScan(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	owned, err := s.QueueScan(ctx, scan)
	if err != nil || owned.Status != "owned" || owned.BookID != added.BookID {
		t.Errorf("a scan of an owned book %+v, err %v", owned, err)
	}

	var fe *FieldError
	if _, err := s.QueueScan(ctx, ScanInput{Code: "070330757855"}); !errors.As(err, &fe) || fe.Message != RetailCodeMessage {
		t.Errorf("a UPC: %v", err)
	}

	unknown, err := s.QueueScan(ctx, ScanInput{Code: "9780000000002"})
	if err != nil {
		t.Fatal(err)
	}
	work()
	if got, _ := s.GetScan(ctx, unknown.ID); got.Status != "not_found" {
		t.Errorf("a book no source knows: %+v", got)
	}

	scans, pending, err := s.OpenScans(ctx)
	if err != nil || pending != 0 || len(scans) != 2 {
		t.Errorf("open scans %d, pending %d, err %v", len(scans), pending, err)
	}

	// Undoing the scan that added a book takes the book away too.
	if err := s.UndoScan(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, added.BookID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the book a scan added outlived its undo: %v", err)
	}
}

func TestIdentify(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "unread", Contributors: []lookup.Contributor{{Name: "Anthony Ryan", Role: "author"}}}, &CopyInput{}, "")
	if err != nil {
		t.Fatal(err)
	}
	for code, want := range map[string]Identified{
		"9780425268285": {Kind: "isbn", BookID: id, Title: "Blood Song", Authors: "Anthony Ryan", Copies: 1},
		"HL00001":       {Kind: "copy", BookID: id, Title: "Blood Song", Authors: "Anthony Ryan", Copies: 1},
		"9780000000002": {Kind: "isbn"},
		"070330757855":  {Kind: "retail"},
	} {
		got, err := s.Identify(ctx, code)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if got.Kind != want.Kind || got.BookID != want.BookID || got.Title != want.Title || got.Authors != want.Authors || got.Copies != want.Copies {
			t.Errorf("%s: %+v", code, got)
		}
	}
}

func TestExport(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "read", Subjects: []string{"Fantasy fiction", "Epic"}, Contributors: []lookup.Contributor{{Name: "Anthony Ryan", Role: "author"}, {Name: "Jane Doe", Role: "illustrator"}}}, &CopyInput{AcquiredAs: "used", PriceCents: 1299, Condition: "good", DustJacket: "unknown"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, BookInput{Title: "No Copy Yet", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, ""); err != nil {
		t.Fatal(err)
	}
	lib, err := s.Export(ctx)
	if err != nil || len(lib.Books) != 2 {
		t.Fatalf("export %d books, err %v", len(lib.Books), err)
	}
	var b strings.Builder
	if err := WriteBooksCSV(&b, lib.Books); err != nil {
		t.Fatal(err)
	}
	text, ok := strings.CutPrefix(b.String(), utf8BOM)
	if !ok {
		t.Error("no byte order mark")
	}
	rows, err := csv.NewReader(strings.NewReader(text)).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows %d, err %v", len(rows), err)
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	var blood []string
	for _, r := range rows[1:] {
		if len(r) != len(rows[0]) {
			t.Errorf("a row of %d cells under %d columns", len(r), len(rows[0]))
		}
		if r[col["Title"]] == "Blood Song" {
			blood = r
		}
	}
	if blood == nil {
		t.Fatal("Blood Song is not in the CSV")
	}
	book, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"Barcode": book.Copies[0].Barcode, "Authors": "Anthony Ryan", "Other contributors": "Jane Doe (illustrator)",
		"Subjects": "Fantasy fiction; Epic", "Price": "12.99", "Bought": "used", "Condition": "good", "Dust jacket": "", "Read status": "read",
	} {
		if got := blood[col[name]]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
