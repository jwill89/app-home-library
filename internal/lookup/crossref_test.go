package lookup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestNormalizeDOI(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"10.1038/NATURE12373", "10.1038/nature12373"},
		{"https://doi.org/10.1038/nature12373", "10.1038/nature12373"},
		{"doi:10.1038/nature12373", "10.1038/nature12373"},
		{"\thttps://dx.doi.org/10.1000/xyz\n", "10.1000/xyz"},
	} {
		if got, ok := NormalizeDOI(c.in); !ok || got != c.want {
			t.Errorf("%q: %q %v, want %q", c.in, got, ok, c.want)
		}
	}
	for _, bad := range []string{"", "nature12373", "10.1/x", "https://example.com/10.1038"} {
		if _, ok := NormalizeDOI(bad); ok {
			t.Errorf("%q taken for a DOI", bad)
		}
	}
}

func TestByDOI(t *testing.T) {
	body, err := os.ReadFile("testdata/crossref-10.1080-00221546.1999.11780773.json")
	if err != nil {
		t.Fatal(err)
	}
	// OpenAlex, which says what it is about: a keyword it is unsure of is left out.
	about := `{"keywords": [{"display_name": "calculus", "score": 0.9}, {"display_name": "noise", "score": 0.1}],
		"primary_topic": {"display_name": "Mathematics Education"}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.UserAgent(), "test") {
			t.Errorf("no user agent: %q", r.UserAgent())
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/works/10.1080/00221546.1999.11780773"):
			_, _ = w.Write(body)
		case strings.HasSuffix(r.URL.Path, "/works/doi:10.1080/00221546.1999.11780773"):
			_, _ = w.Write([]byte(about))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := New(Options{UserAgent: "test", HTTPClient: srv.Client()})
	s.Crossref.BaseURL = srv.URL
	s.OpenAlex.BaseURL = srv.URL

	p, err := s.ByDOI(context.Background(), "10.1080/00221546.1999.11780773", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != "article" || p.Title != "Teaching Calculus for Understanding: A Study of First-Year Students" ||
		p.Container != "The Journal of Higher Education" || p.Volume != "70" || p.Issue != "6" ||
		p.Pages != "623-649" || p.Published != "1999-11" || p.DOI != "10.1080/00221546.1999.11780773" ||
		p.URL != "https://doi.org/10.1080/00221546.1999.11780773" || p.Publisher != "Informa UK Limited" {
		t.Fatalf("proposal %+v", p)
	}
	if !slices.Equal(p.Authors, []string{"Ana María Ruiz", "Ben O'Connor", "The Calculus Study Group"}) {
		t.Fatalf("authors %q", p.Authors)
	}
	if !slices.Equal(p.Subjects, []string{"Calculus", "Mathematics Education"}) {
		t.Fatalf("subjects %q", p.Subjects)
	}
	if p, err := s.ByDOI(context.Background(), "10.1000/missing", false); err != nil || p != nil {
		t.Fatalf("an unknown DOI gave %+v, %v", p, err)
	}
}

func TestParseOpenAlex(t *testing.T) {
	body, err := os.ReadFile("testdata/openalex-10.1093-oso-9780199278183.003.0002.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseOpenAlex(body)
	want := []string{"Causality", "Folk science", "Unity of science", "Natural processes", "Philosophy of science", "Philosophy and History of Science"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("subjects %q, err %v", got, err)
	}
}

func TestAbstracts(t *testing.T) {
	jats := `<jats:title>Abstract</jats:title><jats:p>First point, with <jats:italic>emphasis</jats:italic>.</jats:p><jats:p>Second &amp; last.</jats:p>`
	if got := jatsText(jats); got != "First point, with emphasis.\n\nSecond & last." {
		t.Errorf("jatsText = %q", got)
	}
	body := []byte(`{"abstract_inverted_index":{"Theories":[0],"are":[1,5],"chosen,":[2],"not":[3],"proved;":[4],"rarely":[6],"final.":[7]}}`)
	if got := OpenAlexAbstract(body); got != "Theories are chosen, not proved; are rarely final." {
		t.Errorf("OpenAlexAbstract = %q", got)
	}
	if got := OpenAlexAbstract([]byte(`{"abstract_inverted_index":null}`)); got != "" {
		t.Errorf("no abstract gave %q", got)
	}
}

func TestCrossrefEditors(t *testing.T) {
	chapter := []byte(`{"message":{"type":"book-chapter","title":["Causation as Folk Science"],
		"container-title":["Causation, Physics, and the Constitution of Reality"],
		"author":[{"given":"John D.","family":"Norton"}],
		"editor":[{"given":"Huw","family":"Price"},{"given":"Richard","family":"Corry"}]}}`)
	p, err := parseCrossref(chapter)
	if err != nil || !slices.Equal(p.Editors, []string{"Huw Price", "Richard Corry"}) || !slices.Equal(p.Authors, []string{"John D. Norton"}) {
		t.Errorf("chapter %+v, %v", p, err)
	}
	// A journal's editors are not an article's to cite.
	article := []byte(`{"message":{"type":"journal-article","title":["A"],"editor":[{"given":"Ed","family":"Itor"}]}}`)
	if p, _ := parseCrossref(article); len(p.Editors) != 0 {
		t.Errorf("article editors %v", p.Editors)
	}
}

// A search by title and author gives the works Crossref matches, each as a DOI would fill it;
// one without a DOI or a title is left out.
func TestFindDOIs(t *testing.T) {
	work, err := os.ReadFile("testdata/crossref-10.1080-00221546.1999.11780773.json")
	if err != nil {
		t.Fatal(err)
	}
	var one struct {
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(work, &one); err != nil {
		t.Fatal(err)
	}
	answer := `{"message": {"items": [` + string(one.Message) + `, {"type": "journal-article", "title": ["No DOI"]}]}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/works" || q.Get("query.bibliographic") != "Teaching calculus" || q.Get("query.author") != "Ruiz" {
			t.Errorf("asked %s", r.URL)
		}
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	s := New(Options{UserAgent: "test", HTTPClient: srv.Client()})
	s.Crossref.BaseURL = srv.URL

	found, err := s.FindDOIs(context.Background(), "Teaching calculus", "Ruiz")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].DOI != "10.1080/00221546.1999.11780773" || found[0].Container != "The Journal of Higher Education" ||
		found[0].Published != "1999-11" || len(found[0].Authors) != 3 {
		t.Fatalf("found %+v", found)
	}
}

// A book review is told by its title, which Crossref files as an article.
func TestReviewTitle(t *testing.T) {
	for title, want := range map[string]bool{
		"Book Review: The Dappled World":      true,
		"Review of Causation and Explanation": true,
		"Reviewed Work(s): Leviathan":         true,
		"Review offers a new reading":         false,
		"Book reviewers and their habits":     false,
		"Teaching Calculus":                   false,
	} {
		if got := reviewTitleRe.MatchString(title); got != want {
			t.Errorf("%q: %v, want %v", title, got, want)
		}
	}
}
