package lookup

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNormalizeISBN(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"0374533555", "9780374533557", true},
		{"978-0-374-53355-7", "9780374533557", true},
		{"164275742X", "9781642757422", true},
		{"9780374533558", "", false}, // check digit
		{"0374533556", "", false},
		{"070330757855", "", false},  // a 12-digit UPC
		{"0070330757851", "", false}, // an EAN-13 that is not Bookland
		{"97988884313x0", "", false},
		{"9798888431320", "9798888431320", true},
	} {
		got, ok := NormalizeISBN(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormalizeISBN(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
	if got := ISBN10("9780374533557"); got != "0374533555" {
		t.Errorf("ISBN10 = %q", got)
	}
	if got := ISBN10("9781642757422"); got != "164275742X" {
		t.Errorf("ISBN10 with X = %q", got)
	}
	if got := ISBN10("9798888431320"); got != "" {
		t.Errorf("ISBN10 of a 979 = %q", got)
	}
}

func TestSmallReaders(t *testing.T) {
	for in, want := range map[string]string{"153.4/2": "153.42", "823/.92": "823.92", "741.5/952": "741.5952", "530/.02/02": "530.0202", "[Fic]": "", "B": "", "813.54": "813.54"} {
		if got := NormalizeDDC(in); got != want {
			t.Errorf("NormalizeDDC(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"Kahneman, Daniel,": "Daniel Kahneman", "Ryan, Anthony.": "Anthony Ryan", "King, Martin Luther, Jr.": "Martin Luther King, Jr.", "Kahneman, Daniel, 1934-": "Daniel Kahneman", "Anonymous": "Anonymous"} {
		if got := InvertName(in); got != want {
			t.Errorf("InvertName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := SortName("Martin Luther King, Jr."); got != "King, Martin Luther, Jr." {
		t.Errorf("SortName = %q", got)
	}
	for in, want := range map[string][2]string{"Raven's Shadow, Book 1": {"Raven's Shadow", "1"}, "Raven's mark ; 3": {"Raven's mark", "3"}, "Wotakoi ; v. 4": {"Wotakoi", "4"}, "The Expanse (5)": {"The Expanse", "5"}, "Discworld": {"Discworld", ""},
		// As Open Library has them: the number written into the name.
		"Malazan book of the fallen -- bk. 9":        {"Malazan book of the fallen", "9"},
		"First law trilogy -- book one":              {"First law trilogy", "1"},
		"The Malazan Book of the Fallen: Book 2":     {"The Malazan Book of the Fallen", "2"},
		"The Witcher: Book 0.5":                      {"The Witcher", "0.5"},
		"Book six of the Malazan book of the fallen": {"Malazan book of the fallen", "6"},
		"A Tom Doherty Associates book":              {"A Tom Doherty Associates book", ""},
		"Star Wars -- Legends":                       {"Star Wars -- Legends", ""},
	} {
		if name, n := SplitSeries(in); name != want[0] || n != want[1] {
			t.Errorf("SplitSeries(%q) = %q, %q", in, name, n)
		}
	}
	for in, want := range map[string]string{"pbk.": "paperback", "hardcover : acid-free paper": "hardcover", "alk. paper": "", "(v. 1 ; trade paperback)": "trade_paperback", "Mass Market Paperback": "mass_market", "hc.": "hardcover", "lib. bdg.": "library_binding", "ebook": "", "paperback": "paperback"} {
		if got := FormatFromText(in); got != want {
			t.Errorf("FormatFromText(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"April 2, 2013": "2013-04-02", "Mar 12, 2019": "2019-03-12", "2014": "2014", "[2013]": "2013", "c2013.": "2013", "2019-07-02": "2019-07-02", "2019-07": "2019-07", "": ""} {
		if got := NormalizeDate(in); got != want {
			t.Errorf("NormalizeDate(%q) = %q, want %q", in, got, want)
		}
	}
	if h, w, d := Dimensions("7.9 x 5.2 x 0.5 inches"); h != 201 || w != 132 || d != 13 {
		t.Errorf("Dimensions inches = %d %d %d", h, w, d)
	}
	if h, w, d := Dimensions("14 x 2 x 21 centimeters"); h != 210 || w != 140 || d != 20 {
		t.Errorf("Dimensions cm = %d %d %d", h, w, d)
	}
}

func TestParseLoC(t *testing.T) {
	recs, err := ParseLoC(fixture(t, "loc-9780374533557.xml"), "9780374533557")
	if err != nil || len(recs) != 1 {
		t.Fatalf("records = %d, err %v", len(recs), err)
	}
	r := recs[0]
	if r.Title != "Thinking, fast and slow" || r.DDC != "153.42" || r.LCC != "BF441 .K238 2013" {
		t.Errorf("title %q, ddc %q, lcc %q", r.Title, r.DDC, r.LCC)
	}
	if r.Format != "paperback" || r.HeightMM != 240 || r.Pages != 499 || r.Edition != "1st pbk. ed" {
		t.Errorf("format %q, height %d, pages %d, edition %q", r.Format, r.HeightMM, r.Pages, r.Edition)
	}
	if r.Publisher != "Farrar, Straus and Giroux" || r.Place != "New York" || r.Published != "2013" || r.Language != "en" {
		t.Errorf("publisher %q, place %q, published %q, language %q", r.Publisher, r.Place, r.Published, r.Language)
	}
	if len(r.Contributors) == 0 || r.Contributors[0] != (Contributor{"Daniel Kahneman", "author"}) {
		t.Errorf("contributors %v", r.Contributors)
	}
	if !slices.Contains(r.Subjects, "Thought and thinking") {
		t.Errorf("subjects %v", r.Subjects)
	}

	none, err := ParseLoC(fixture(t, "loc-9780425268285.xml"), "9780425268285")
	if err != nil || len(none) != 0 {
		t.Errorf("a book the catalog lacks: %d records, err %v", len(none), err)
	}

	// One record for a whole series: the binding and volume are the scanned ISBN's own.
	series, err := ParseLoC(fixture(t, "loc-9781975309725.xml"), "9781975309725")
	if err != nil || len(series) != 1 {
		t.Fatalf("series records = %d, err %v", len(series), err)
	}
	if s := series[0]; s.Format != "trade_paperback" || s.SeriesNumber != "1" || s.DDC != "" || s.HeightMM != 210 {
		t.Errorf("format %q, number %q, ddc %q, height %d", s.Format, s.SeriesNumber, s.DDC, s.HeightMM)
	}

	related, err := ParseLoC(fixture(t, "loc-search-blood-song.xml"), "")
	if err != nil || len(related) != 1 || related[0].DDC != "823.92" || related[0].Format != "hardcover" {
		t.Errorf("related %+v, err %v", related, err)
	}
}

func TestParseOpenLibrary(t *testing.T) {
	body, err := combine(fixture(t, "ol-data-9780425268285.json"), fixture(t, "ol-details-9780425268285.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseOpenLibrary(body, "https://covers.openlibrary.org")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title != "Blood Song" || r.Format != "mass_market" || r.Series != "Raven's Shadow" || r.SeriesNumber != "1" || r.Pages != 624 {
		t.Errorf("title %q, format %q, series %q #%q, pages %d", r.Title, r.Format, r.Series, r.SeriesNumber, r.Pages)
	}
	// The cover as uploaded, not Open Library's "large", which is cut to 500 pixels high.
	covers := []string{"https://covers.openlibrary.org/b/id/10423158.jpg"}
	if r.LCC != "PR6118.Y3523 B56 2014" || !slices.Equal(r.Covers, covers) || len(r.Contributors) == 0 || r.Contributors[0].Name != "Anthony Ryan" {
		t.Errorf("lcc %q, covers %v, contributors %v", r.LCC, r.Covers, r.Contributors)
	}

	body, err = combine(fixture(t, "ol-data-9781593082666.json"), fixture(t, "ol-details-9781593082666.json"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseOpenLibrary(body, "https://covers.openlibrary.org")
	if err != nil {
		t.Fatal(err)
	}
	if b.HeightMM != 201 || b.WidthMM != 132 || b.ThicknessMM != 13 || b.Format != "paperback" {
		t.Errorf("height %d, width %d, thickness %d, format %q", b.HeightMM, b.WidthMM, b.ThicknessMM, b.Format)
	}
}

func TestParseOpenLibrarySearch(t *testing.T) {
	got, err := parseOpenLibrarySearch(fixture(t, "ol-search-sponger.json"), "https://covers.openlibrary.org")
	if err != nil || len(got) != 3 {
		t.Fatalf("candidates %d, err %v", len(got), err)
	}
	if c := got[0]; c.ISBN13 != "9781642750430" || c.Publisher != "Seven Seas" || c.Published != "2019-03-12" {
		t.Errorf("first candidate %+v", c)
	}
	// A small picture to recognise it by, and the cover to keep at its full size.
	if c := got[0]; !strings.HasSuffix(c.Thumbnail, "-M.jpg") || len(c.Covers) != 1 || c.Covers[0].URL != strings.TrimSuffix(c.Thumbnail, "-M.jpg")+".jpg" {
		t.Errorf("thumbnail %q, covers %v", c.Thumbnail, c.Covers)
	}
}

func TestParseGoogle(t *testing.T) {
	body := []byte(`{"items":[
		{"volumeInfo":{"title":"Other","industryIdentifiers":[{"type":"ISBN_13","identifier":"9780000000002"}]}},
		{"volumeInfo":{"title":"Thinking, Fast and Slow","authors":["Daniel Kahneman"],"publisher":"Farrar, Straus and Giroux","publishedDate":"2013-04-02","description":"<p>Two systems.</p>","pageCount":512,"categories":["Psychology"],"language":"en",
		"industryIdentifiers":[{"type":"ISBN_10","identifier":"0374533555"},{"type":"ISBN_13","identifier":"9780374533557"}],
		"imageLinks":{"thumbnail":"http://books.google.com/books/content?id=abc&printsec=frontcover&img=1&zoom=1&edge=curl&source=gbs_api"}}}]}`)
	r, err := ParseGoogle(body, "9780374533557")
	if err != nil || r == nil {
		t.Fatalf("record %v, err %v", r, err)
	}
	// Google's covers cannot be fetched from the droplet, so a record never offers one.
	if r.Title != "Thinking, Fast and Slow" || r.Description != "Two systems." || len(r.Covers) != 0 {
		t.Errorf("title %q, description %q, covers %v", r.Title, r.Description, r.Covers)
	}
	if got := thumbnail("http://books.google.com/books/content?id=abc&img=1&edge=curl"); got != "https://books.google.com/books/content?id=abc&img=1" {
		t.Errorf("thumbnail %q", got)
	}
	if miss, _ := ParseGoogle(body, "9781111111113"); miss != nil {
		t.Errorf("a volume without the ISBN was taken: %+v", miss)
	}
	// A volume's whole record has its measurements, in centimetres.
	whole := []byte(`{"items":[{"id":"x","volumeInfo":{"title":"Sweat and Soap 2","industryIdentifiers":[{"type":"ISBN_13","identifier":"9781632369710"}],
		"dimensions":{"height":"20.80 cm","width":"14.60 cm","thickness":"1.80 cm"}}}]}`)
	if r, _ := ParseGoogle(whole, "9781632369710"); r == nil || r.HeightMM != 208 || r.WidthMM != 146 || r.ThicknessMM != 18 {
		t.Errorf("measurements %+v", r)
	}
	for in, want := range map[string]int{"20.80 cm": 208, "7.5 in": 191, "": 0, "0.00 cm": 0, "tall": 0} {
		if got := googleLength(in); got != want {
			t.Errorf("googleLength(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestGoogleFetchesTheWholeVolume(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/volumes":
			_, _ = fmt.Fprint(w, `{"items":[{"id":"other","volumeInfo":{"industryIdentifiers":[{"type":"ISBN_13","identifier":"9780000000002"}]}},
				{"id":"abc","volumeInfo":{"industryIdentifiers":[{"type":"ISBN_13","identifier":"9781632369710"}]}}]}`)
		case "/volumes/abc":
			_, _ = fmt.Fprint(w, `{"id":"abc","volumeInfo":{"title":"Sweat and Soap 2","industryIdentifiers":[{"type":"ISBN_13","identifier":"9781632369710"}],"dimensions":{"height":"20.80 cm"}}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	g := &Google{BaseURL: srv.URL, Key: "k", fetch: newFetcher(srv.Client(), "test", 0)}
	body, err := g.FetchISBN(t.Context(), "9781632369710")
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := ParseGoogle(body, "9781632369710"); r == nil || r.Title != "Sweat and Soap 2" || r.HeightMM != 208 {
		t.Errorf("record %+v from %s", r, body)
	}
}

func TestMerge(t *testing.T) {
	recs := map[string]*Record{
		SourceGoogle:      {Title: "Thinking, Fast and Slow", Contributors: []Contributor{{"Daniel Kahneman", "author"}}, Published: "2013", Pages: 512, Subjects: []string{"Psychology"}, Covers: []string{"g"}},
		SourceOpenLibrary: {Title: "Thinking, fast and slow", Format: "paperback", Published: "2013-04-02", Pages: 499, Covers: []string{"o"}},
		SourceHardcover:   {Covers: []string{"h"}},
		SourceLoC:         {Title: "Thinking, fast and slow", Format: "mass_market", DDC: "153.42", HeightMM: 240, Subjects: []string{"Thought and thinking"}, Contributors: []Contributor{{"Daniel Kahneman", "author"}, {"Jane Doe", "translator"}}},
	}
	p := Merge("9780374533557", recs)
	if p.Title != "Thinking, Fast and Slow" || p.Sources["title"] != SourceGoogle {
		t.Errorf("title %q from %q", p.Title, p.Sources["title"])
	}
	if p.Published != "2013-04-02" || p.Pages != 499 || p.DDC != "153.42" || p.HeightMM != 240 {
		t.Errorf("published %q, pages %d, ddc %q, height %d", p.Published, p.Pages, p.DDC, p.HeightMM)
	}
	if p.Format != "mass_market" || p.Sources["format"] != SourceLoC {
		t.Errorf("format %q from %q", p.Format, p.Sources["format"])
	}
	if len(p.Contributors) != 2 || p.Contributors[1].Role != "translator" {
		t.Errorf("contributors %v", p.Contributors)
	}
	// Google's covers are left out: its image host refuses the server.
	covers := []CoverChoice{{URL: "o", Source: SourceOpenLibrary}, {URL: "h", Source: SourceHardcover}}
	if !slices.Equal(p.Subjects, []string{"Thought and thinking", "Psychology"}) || !slices.Equal(p.Covers, covers) {
		t.Errorf("subjects %v, covers %v", p.Subjects, p.Covers)
	}
	if p.ISBN10 != "0374533555" {
		t.Errorf("isbn10 %q", p.ISBN10)
	}
}

func combine(data, details []byte) ([]byte, error) {
	return []byte(`{"data":` + string(data) + `,"details":` + string(details) + `}`), nil
}

func TestPlainText(t *testing.T) {
	// The Bookshelf export's encoding, with a stray close tag, a list, and an entity.
	in := "&&LDIV&&R&&LI&&RThe Metamorphoses&&L/I&&R, by &&LSTRONG&&ROvid&&L/B&&R, is part of the series:&&LDIV&&R\n" +
		"New introductions\nBiographies &amp; chronologies&&L/DIV&&R&&LP style=\"MARGIN: 0in\"&&R &&L/P&&R&&LP&&RFirst published in 8 A.D.&&L/P&&R"
	want := "The Metamorphoses, by Ovid, is part of the series:\nNew introductions\nBiographies & chronologies\n\nFirst published in 8 A.D."
	if got := PlainText(in); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if got := PlainText("A plain line.\n\nAnother, with 3 < 4."); got != "A plain line.\n\nAnother, with 3 < 4." {
		t.Fatalf("plain text changed: %q", got)
	}
	if got := PlainText("<p>One<br/>two</p><ul><li>three</li><li>four</li></ul>"); got != "One\ntwo\n\nthree\nfour" {
		t.Fatalf("html gave %q", got)
	}
}
