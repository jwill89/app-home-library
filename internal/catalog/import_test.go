package catalog

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"homelibrary/internal/lookup"
)

const goodreadsCSV = `Book Id,Title,Author,Author l-f,Additional Authors,ISBN,ISBN13,My Rating,Average Rating,Publisher,Binding,Number of Pages,Year Published,Original Publication Year,Date Read,Date Added,Bookshelves,Bookshelves with positions,Exclusive Shelf,My Review,Spoiler,Private Notes,Read Count,Owned Copies
13569581,"Blood Song (Raven's Shadow, #1)",Anthony Ryan,"Ryan, Anthony",,"=""0425268284""","=""9780425268285""",5,4.42,Ace,Paperback,624,2014,2011,2015/03/02,2015/01/10,"fantasy, epic","fantasy (#3), epic (#1)",read,Loved it.,,,1,1
1,Wanted Book,Some Author,"Author, Some",,"=""""","=""9780306406157""",0,3.9,Press,Hardcover,300,2020,2020,,2024/01/01,to-read,to-read (#4),to-read,,,,0,0
2,Library Loan,Other Author,"Author, Other",,"=""""","=""9780131103627""",3,4.1,Press,Hardcover,272,1988,1978,2022/06/01,2022/05/01,,,read,,,,1,0
3,"Thinking, Fast and Slow",Daniel Kahneman,"Kahneman, Daniel",,"=""""","=""9780374533557""",4,4.2,FSG,Paperback,499,2013,2011,2020/08/09,2020/07/01,"psychology, owned","psychology (#1), owned (#2)",read,,,A note.,1,0
`

func TestImportGoodreads(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	// One book the library has already, unread, typed in by hand.
	kahneman, err := s.Create(ctx, BookInput{Title: "Thinking, Fast and Slow", ISBN: "9780374533557", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}

	rep, err := s.ImportFile(ctx, strings.NewReader(goodreadsCSV), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Source != "Goodreads" || rep.Rows != 4 || rep.Added != 1 || rep.Matched != 1 || rep.Wanted != 1 || rep.NotOwned != 1 || rep.Queued != 2 || len(rep.Problems) != 0 {
		t.Fatalf("report %+v", rep)
	}
	id, err := s.FindByISBN(ctx, "9780425268285")
	if err != nil || id == 0 {
		t.Fatalf("Blood Song not added: %v", err)
	}
	blood, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if blood.Title != "Blood Song" || blood.Series != "Raven's Shadow" || blood.SeriesNumber != "1" || blood.ReadStatus != "read" ||
		blood.FinishedOn != "2015-03-02" || blood.Rating != 5 || blood.Notes != "Loved it." || blood.Format != "paperback" ||
		!slices.Equal(blood.Genres, []string{"epic", "fantasy"}) || blood.Sources["title"] != ImportSourceGoodreads {
		t.Errorf("Blood Song %+v", blood)
	}
	if len(blood.Links) != 1 || blood.Links[0].URL != "https://www.goodreads.com/book/show/13569581" {
		t.Errorf("links %+v", blood.Links)
	}

	// The book the library had keeps its own details and gains the reading it lacked.
	k, err := s.Get(ctx, kahneman)
	if err != nil {
		t.Fatal(err)
	}
	if k.ReadStatus != "read" || k.Rating != 4 || k.FinishedOn != "2020-08-09" || k.Notes != "A note." || len(k.Links) != 1 {
		t.Errorf("the matched book %+v", k)
	}

	// The to-read book is on the wishlist, with no copy.
	wantedID, _ := s.FindByISBN(ctx, "9780306406157")
	if w, err := s.Get(ctx, wantedID); err != nil || w.WantedOn == "" || len(w.Copies) != 0 {
		t.Errorf("the to-read book %+v, err %v", w, err)
	}

	// Twice is once; asking for everything adds the library loan to the shelves.
	again, err := s.ImportFile(ctx, strings.NewReader(goodreadsCSV), ImportOptions{NotOwned: "library"})
	if err != nil || again.Already != 2 || again.Matched != 1 || again.Added != 1 || again.NotOwned != 0 || again.Wanted != 0 {
		t.Fatalf("again %+v, err %v", again, err)
	}
}

func TestImportLibraryThing(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	header := []string{"Book Id", "Title", "Sort Character", "Primary Author", "Primary Author Role", "Secondary Author", "Secondary Author Roles", "Publication", "Date", "Review", "Rating", "Comment", "Private Comment", "Summary", "Media", "Physical Description", "Weight", "Height", "Thickness", "Length", "Dimensions", "Page Count", "LCCN", "Acquired", "Date Started", "Date Read", "Barcode", "BCID", "Tags", "Collections", "Languages", "Original Languages", "LC Classification", "ISBN", "ISBNs", "Subjects", "Dewey Decimal", "Dewey Wording", "Other Call Number", "Copies", "Source", "Entry Date", "From Where", "OCLC", "Work id"}
	wind := map[string]string{
		"Book Id": "1001", "Title": "The Name of the Wind", "Primary Author": "Rothfuss, Patrick", "Primary Author Role": "Author",
		"Secondary Author": "Doe, Jane", "Secondary Author Roles": "Illustrator", "Publication": "DAW (2007), Edition: 1, 662 pages",
		"Date": "2007", "Rating": "4.5", "Media": "Hardcover", "Height": "9.25 inches", "Page Count": "662", "Acquired": "2019-01-02",
		"Date Read": "2019-02-03", "Tags": "fantasy, favorites", "Collections": "Your library, Favorites", "Languages": "English",
		"ISBN": "[0756404746]", "ISBNs": "0756404746, 9780756404741", "Dewey Decimal": "813.6", "Copies": "1", "Work id": "2371329",
	}
	wish := map[string]string{"Book Id": "1002", "Title": "Someday", "Primary Author": "Writer, A.", "Collections": "Wishlist", "ISBNs": "9780306406157"}
	var lines []string
	lines = append(lines, strings.Join(header, "\t"))
	for _, row := range []map[string]string{wind, wish} {
		cells := make([]string, len(header))
		for i, h := range header {
			cells[i] = row[h]
		}
		lines = append(lines, strings.Join(cells, "\t"))
	}
	rep, err := s.ImportFile(ctx, strings.NewReader(strings.Join(lines, "\n")+"\n"), ImportOptions{NotOwned: "library"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Source != "LibraryThing" || rep.Rows != 2 || rep.Added != 1 || rep.Wanted != 1 || rep.NotOwned != 0 || len(rep.Problems) != 0 {
		t.Fatalf("report %+v", rep)
	}
	id, err := s.FindByISBN(ctx, "9780756404741")
	if err != nil || id == 0 {
		t.Fatalf("not added: %v", err)
	}
	b, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := []lookup.Contributor{{Name: "Patrick Rothfuss", Role: "author"}, {Name: "Jane Doe", Role: "illustrator"}}
	if b.Publisher != "DAW" || b.Published != "2007" || b.Rating != 4 || b.ReadStatus != "read" || b.FinishedOn != "2019-02-03" ||
		b.HeightMM != 235 || b.DDC != "813.6" || b.Format != "hardcover" || b.Language != "en" || !slices.Equal(b.Contributors, want) ||
		b.Copies[0].AcquiredOn != "2019-01-02" {
		t.Errorf("book %+v", b)
	}
	if len(b.Links) != 1 || b.Links[0].URL != "https://www.librarything.com/work/2371329/details/1001" {
		t.Errorf("links %+v", b.Links)
	}
}

func TestImportUnknownFile(t *testing.T) {
	s := newService(t)
	var fe *FieldError
	_, err := s.ImportFile(context.Background(), strings.NewReader("Name,Price\nTea,3\n"), ImportOptions{})
	if !errors.As(err, &fe) || fe.Field != "file" {
		t.Errorf("a file from no known app: %v", err)
	}
}

func TestWishlist(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "unread"}, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Get(ctx, id)
	if err != nil || b.WantedOn == "" || len(b.Copies) != 0 {
		t.Fatalf("a wanted book %+v, err %v", b, err)
	}
	rows, _ := s.List(ctx)
	if len(rows) != 1 || rows[0].WantedOn == "" || rows[0].Copies != 0 {
		t.Errorf("the list's row %+v", rows)
	}

	// Scanned at a bookstore, it is named as wanted.
	sc, err := s.QueueScan(ctx, ScanInput{Code: "9780425268285"})
	if err != nil || sc.Status != "owned" || sc.WantedOn == "" {
		t.Errorf("a scan of a wanted book %+v, err %v", sc, err)
	}

	// Its first copy takes it off the wishlist.
	if _, err := s.AddCopy(ctx, id, CopyInput{PriceCents: -1}); err != nil {
		t.Fatal(err)
	}
	if b, _ = s.Get(ctx, id); b.WantedOn != "" || len(b.Copies) != 1 {
		t.Errorf("after its first copy %+v", b)
	}
}

func TestSeries(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	add := func(title, series, number string, wanted bool) {
		t.Helper()
		first := &CopyInput{PriceCents: -1}
		if wanted {
			first = nil
		}
		if _, err := s.Create(ctx, BookInput{Title: title, Series: series, SeriesNumber: number, ReadStatus: "unread"}, first, ""); err != nil {
			t.Fatal(err)
		}
	}
	add("Sponger 1", "The Ideal Sponger Life", "1", false)
	add("Sponger 2", "The Ideal Sponger Life", "2", false)
	add("Sponger 5", "The Ideal Sponger Life", "5", false)
	add("Sponger 4", "The Ideal Sponger Life", "4", true)
	add("Sponger 3", "漫画 理想のヒモ生活", "3", false)

	list, err := s.SeriesList(ctx, nil)
	if err != nil || len(list) != 2 {
		t.Fatalf("series %+v, err %v", list, err)
	}
	en := list[0]
	if en.Name != "The Ideal Sponger Life" {
		en = list[1]
	}
	if en.Books != 3 || !slices.Equal(en.Had, []string{"1", "2", "5"}) || !slices.Equal(en.Wanted, []string{"4"}) ||
		!slices.Equal(en.Gaps, []string{"3"}) || en.Next != "6" {
		t.Errorf("the English series %+v", en)
	}

	// Renamed to the other's name, the Japanese-named series merges into it.
	var jp Series
	for _, x := range list {
		if x.ID != en.ID {
			jp = x
		}
	}
	keep, err := s.RenameSeries(ctx, jp.ID, SeriesInput{Name: "the ideal sponger life"})
	if err != nil || keep != en.ID {
		t.Fatalf("merge kept %d, err %v", keep, err)
	}
	if list, _ = s.SeriesList(ctx, nil); len(list) != 1 || list[0].Books != 4 || len(list[0].Gaps) != 0 {
		t.Errorf("after the merge %+v", list)
	}
	if _, err := s.RenameSeries(ctx, keep, SeriesInput{Name: "Ideal Sponger Life (Manga)"}); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.SeriesList(ctx, nil); list[0].Name != "Ideal Sponger Life (Manga)" {
		t.Errorf("rename %+v", list)
	}
}
