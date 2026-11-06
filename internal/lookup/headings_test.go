package lookup

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEnglishSubjects(t *testing.T) {
	// What the subject authority says, by the words asked; anything else is no heading.
	answers := map[string]string{
		"socialism":           `{"hits":[{"aLabel":"Socialism","vLabel":""},{"aLabel":"Socialism--Africa","vLabel":""}]}`,
		"revolutions":         `{"hits":[{"aLabel":"Revolutions","vLabel":""}]}`,
		"socialist movements": `{"hits":[{"aLabel":"Socialism","vLabel":"Socialist movements"}]}`,
		"manga":               `{"hits":[{"aLabel":"Manga (Chad and Niger)","vLabel":""}]}`,
	}
	asked := 0
	client := &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		asked++
		body, ok := answers[r.URL.Query().Get("q")]
		if !ok {
			body = `{"hits":[]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	s := New(Options{UserAgent: "test", HTTPClient: client, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	s.Headings.fetch.gap = 0

	got := s.englishSubjects(context.Background(), []string{
		"Socialism",
		"Socialisme",
		"Revolutie",
		"Rossiĭskai︠a︡ sot︠s︡ial-demokraticheskai︠a︡ rabochai︠a︡ partii︠a︡",
		"Socialist movements",
		"Revolutions -- History",
		"Manga",
		"socialism.",
	})
	if want := []string{"Socialism", "Revolutions -- History"}; !slices.Equal(got, want) {
		t.Errorf("kept %q, want %q", got, want)
	}
	if asked != 8 {
		t.Errorf("asked the authority %d times", asked)
	}

	// The catalog's and Google's subjects are kept as they come; Open Library's are marked
	// for the check, used only when neither has any.
	only := Merge("9780717802180", map[string]*Record{SourceOpenLibrary: {Title: "What Is to Be Done?", Subjects: []string{"Socialism", "Socialisme"}}})
	if only.Sources["subjects"] != SourceOpenLibrary {
		t.Errorf("Open Library's subjects marked %q", only.Sources["subjects"])
	}
	both := Merge("9780717802180", map[string]*Record{
		SourceOpenLibrary: {Subjects: []string{"Socialisme"}},
		SourceGoogle:      {Title: "What Is to Be Done?", Subjects: []string{"Political Science"}},
	})
	if both.Sources["subjects"] != "merged" || !slices.Equal(both.Subjects, []string{"Political Science"}) {
		t.Errorf("with Google's: %q from %q", both.Subjects, both.Sources["subjects"])
	}
}
