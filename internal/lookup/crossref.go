package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// CrossrefURL is Crossref's free REST API, which knows the DOIs of most journal articles and
// many chapters. It asks for no key; the app's name and address are in every request.
const CrossrefURL = "https://api.crossref.org"

// Crossref looks DOIs up.
type Crossref struct {
	BaseURL string
	fetch   *fetcher
}

// DocProposal is what Crossref says a DOI is, and OpenAlex what it is about, or what a
// library's exported citation says, in the shape a document's form takes.
type DocProposal struct {
	Kind      string   `json:"kind" enum:"article,chapter,review,other"`
	Title     string   `json:"title"`
	Authors   []string `json:"authors"`
	Editors   []string `json:"editors" doc:"A chapter's book's editors"`
	Container string   `json:"container"`
	Volume    string   `json:"volume"`
	Issue     string   `json:"issue"`
	Pages     string   `json:"pages"`
	Published string   `json:"published" doc:"YYYY, YYYY-MM, or YYYY-MM-DD"`
	Publisher string   `json:"publisher"`
	DOI       string   `json:"doi"`
	URL       string   `json:"url"`
	Subjects  []string `json:"subjects"`
	Abstract  string   `json:"abstract" doc:"Crossref's, OpenAlex's where Crossref has none, or an exported citation's"`
}

var doiRe = regexp.MustCompile(`^10\.\d{4,9}/\S+$`)

// NormalizeDOI takes a DOI as it is pasted (bare, as doi:…, or as a doi.org link) and returns
// it bare and in lower case, which is how DOIs compare.
func NormalizeDOI(s string) (string, bool) {
	s = strings.TrimSpace(s)
	for _, p := range []string{"https://doi.org/", "http://doi.org/", "https://dx.doi.org/", "http://dx.doi.org/", "doi.org/", "doi:"} {
		if len(s) > len(p) && strings.EqualFold(s[:len(p)], p) {
			s = s[len(p):]
		}
	}
	s = strings.ToLower(strings.TrimSpace(s))
	return s, doiRe.MatchString(s)
}

// ByDOI asks Crossref about a DOI, and OpenAlex what it is about and, where Crossref has
// none, its abstract. It returns nil and no error when Crossref has no such DOI; OpenAlex not
// answering only leaves out what it would give. fresh asks both again rather than taking what
// they said before.
func (s *Service) ByDOI(ctx context.Context, doi string, fresh bool) (*DocProposal, error) {
	body, err := s.cached(ctx, "doi:"+doi, "crossref", fresh, func(ctx context.Context) ([]byte, error) {
		return s.Crossref.fetch.get(ctx, s.Crossref.BaseURL+"/works/"+url.PathEscape(doi))
	})
	if err != nil {
		return nil, fmt.Errorf("crossref: %w", err)
	}
	if len(body) == 0 {
		return nil, nil
	}
	p, err := parseCrossref(body)
	if err != nil {
		return nil, err
	}
	about, abstract, err := s.aboutDOI(ctx, doi, fresh)
	if err != nil {
		s.logger.Warn("doi subjects", "doi", doi, "err", err)
	}
	if p.Abstract == "" {
		p.Abstract = abstract
	}
	// OpenAlex's first: Crossref's are the journal's broad field ("General Medicine").
	subjects := about
	for _, c := range p.Subjects {
		subjects = appendUnique(subjects, c)
	}
	p.Subjects = subjects
	return p, nil
}

type crossrefWork struct {
	Message crossrefMessage `json:"message"`
}

// crossrefMessage is one work as Crossref describes it, alone or among a search's items.
type crossrefMessage struct {
	Type      string         `json:"type"`
	Title     []string       `json:"title"`
	Subtitle  []string       `json:"subtitle"`
	Container []string       `json:"container-title"`
	Volume    string         `json:"volume"`
	Issue     string         `json:"issue"`
	Page      string         `json:"page"`
	Abstract  string         `json:"abstract"`
	Subject   []string       `json:"subject"`
	Publisher string         `json:"publisher"`
	DOI       string         `json:"DOI"`
	URL       string         `json:"URL"`
	Author    []crossrefName `json:"author"`
	Editor    []crossrefName `json:"editor"`
	Issued    struct {
		DateParts [][]int `json:"date-parts"`
	} `json:"issued"`
}

// crossrefName is a person as Crossref gives one: given and family names, or a body's name.
type crossrefName struct {
	Given  string `json:"given"`
	Family string `json:"family"`
	Name   string `json:"name"`
}

func (n crossrefName) String() string {
	if name := clean(strings.TrimSpace(n.Given + " " + n.Family)); name != "" {
		return name
	}
	return clean(n.Name) // a body, as author or editor
}

// reviewTitleRe is a title that says it reviews a book: "Book Review: …", "Review of …",
// "Reviewed Work(s): …", as journals and JSTOR head them.
var reviewTitleRe = regexp.MustCompile(`(?i)^(book reviews?\b|review of\b|reviewed works?\b)`)

// tags strips the JATS markup Crossref leaves in some titles (<i>, <sub>, <scp>).
var tags = regexp.MustCompile(`<[^>]+>`)

// jatsTitleRe is a heading in a JATS abstract ("Abstract", "Summary"), which the field says.
var jatsTitleRe = regexp.MustCompile(`(?s)<jats:title[^>]*>.*?</jats:title>`)

// jatsText reads an abstract as Crossref gives it, in JATS markup, as paragraphs of text.
func jatsText(s string) string {
	s = jatsTitleRe.ReplaceAllString(s, "")
	s = strings.NewReplacer("<jats:p>", "<p>", "</jats:p>", "</p>", "<jats:sec>", "", "</jats:sec>", "").Replace(s)
	return PlainText(s)
}

// clean is Crossref's text as it reads: its markup gone, its entities ("&amp;") the characters
// they stand for, and its spacing single.
func clean(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tags.ReplaceAllString(s, ""))), " ")
}

func parseCrossref(body []byte) (*DocProposal, error) {
	var w crossrefWork
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("crossref answer: %w", err)
	}
	return crossrefProposal(w.Message), nil
}

// FindDOIs asks Crossref for the works that best match a title, and an author when one is
// known, best first: a paper whose DOI the owner does not have. Only what tells one from
// another is asked for, not the abstracts.
func (s *Service) FindDOIs(ctx context.Context, title, author string) ([]DocProposal, error) {
	q := url.Values{
		"query.bibliographic": {title},
		"rows":                {"6"},
		"select":              {"DOI,type,title,subtitle,author,editor,container-title,issued,publisher,volume,issue,page"},
	}
	if author != "" {
		q.Set("query.author", author)
	}
	body, err := s.Crossref.fetch.get(ctx, s.Crossref.BaseURL+"/works?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("crossref search: %w", err)
	}
	var found struct {
		Message struct {
			Items []crossrefMessage `json:"items"`
		} `json:"message"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &found); err != nil {
			return nil, fmt.Errorf("crossref search answer: %w", err)
		}
	}
	out := []DocProposal{}
	for _, m := range found.Message.Items {
		if p := crossrefProposal(m); p.DOI != "" && p.Title != "" {
			out = append(out, *p)
		}
	}
	return out, nil
}

func crossrefProposal(m crossrefMessage) *DocProposal {
	p := &DocProposal{
		Kind: "other", Volume: clean(m.Volume), Issue: clean(m.Issue), Pages: strings.ReplaceAll(clean(m.Page), "–", "-"),
		Publisher: clean(m.Publisher), DOI: strings.ToLower(m.DOI), Authors: []string{}, Editors: []string{}, Subjects: []string{},
		Abstract: jatsText(m.Abstract),
	}
	for _, sub := range m.Subject {
		p.Subjects = appendUnique(p.Subjects, clean(sub))
	}
	switch m.Type {
	case "journal-article", "proceedings-article", "posted-content", "report", "dissertation":
		p.Kind = "article"
	case "book-chapter", "book-section", "book-part", "reference-entry":
		p.Kind = "chapter"
	}
	// Crossref has no kind for a book review, which it files as an article; one whose title
	// says so is one.
	if p.Kind == "article" && len(m.Title) > 0 && reviewTitleRe.MatchString(clean(m.Title[0])) {
		p.Kind = "review"
	}
	if len(m.Title) > 0 {
		p.Title = clean(m.Title[0])
		if len(m.Subtitle) > 0 && m.Subtitle[0] != "" {
			p.Title += ": " + clean(m.Subtitle[0])
		}
	}
	if len(m.Container) > 0 {
		p.Container = clean(m.Container[0])
	}
	for _, a := range m.Author {
		if name := a.String(); name != "" {
			p.Authors = append(p.Authors, name)
		}
	}
	// A chapter's editors are its book's; an article's, a journal's, are not cited.
	if p.Kind == "chapter" {
		for _, e := range m.Editor {
			if name := e.String(); name != "" {
				p.Editors = append(p.Editors, name)
			}
		}
	}
	if len(m.Issued.DateParts) > 0 {
		var parts []string
		for i, n := range m.Issued.DateParts[0] {
			if i == 0 {
				parts = append(parts, strconv.Itoa(n))
			} else {
				parts = append(parts, fmt.Sprintf("%02d", n))
			}
		}
		p.Published = strings.Join(parts, "-")
	}
	if p.DOI != "" {
		p.URL = "https://doi.org/" + p.DOI
	}
	return p
}
