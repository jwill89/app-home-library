package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OpenAlexURL is OpenAlex's free API, an open index of scholarly works. It asks for no key.
const OpenAlexURL = "https://api.openalex.org"

// SourceOpenAlex names OpenAlex in the lookup's cache.
const SourceOpenAlex = "openalex"

// OpenAlex says what a DOI is about: the keywords its publisher or OpenAlex gave it, and its
// topic. Crossref, which gives the rest, names subjects for few journals and no chapters.
type OpenAlex struct {
	BaseURL string
	fetch   *fetcher
}

// minKeywordScore leaves out the keywords OpenAlex is least sure of.
const minKeywordScore = 0.4

// openAlexWork is OpenAlex's record of a DOI; empty when it does not know it. fresh asks
// again rather than taking what it said before.
func (s *Service) openAlexWork(ctx context.Context, doi string, fresh bool) ([]byte, error) {
	body, err := s.cached(ctx, "doi:"+doi, SourceOpenAlex, fresh, func(ctx context.Context) ([]byte, error) {
		return s.OpenAlex.fetch.get(ctx, s.OpenAlex.BaseURL+"/works/doi:"+url.PathEscape(doi))
	})
	if err != nil {
		return nil, fmt.Errorf("openalex: %w", err)
	}
	return body, nil
}

// aboutDOI asks OpenAlex what a DOI is about, and its abstract; nothing, and no error, when it
// does not know it.
func (s *Service) aboutDOI(ctx context.Context, doi string, fresh bool) (subjects []string, abstract string, err error) {
	body, err := s.openAlexWork(ctx, doi, fresh)
	if err != nil || len(body) == 0 {
		return nil, "", err
	}
	if subjects, err = ParseOpenAlex(body); err != nil {
		return nil, "", err
	}
	return subjects, OpenAlexAbstract(body), nil
}

// OpenAlexAbstract puts a work's abstract back in order: OpenAlex gives it as each word with
// the places it stands in. Empty when it has none.
func OpenAlexAbstract(body []byte) string {
	var w struct {
		Index map[string][]int `json:"abstract_inverted_index"`
	}
	if json.Unmarshal(body, &w) != nil || len(w.Index) == 0 {
		return ""
	}
	last := -1
	for _, places := range w.Index {
		for _, p := range places {
			last = max(last, p)
		}
	}
	if last < 0 || last > 20000 {
		return ""
	}
	words := make([]string, last+1)
	for word, places := range w.Index {
		for _, p := range places {
			if p >= 0 {
				words[p] = word
			}
		}
	}
	return strings.Join(strings.Fields(strings.Join(words, " ")), " ")
}

// OpenCopy is where a work can be read without a subscription, as OpenAlex knows it: an
// author's manuscript in a repository, as often as the publisher's own open version.
type OpenCopy struct {
	PDFURL  string `json:"pdfUrl" doc:"The PDF itself; empty when only the page it is on is known"`
	PageURL string `json:"pageUrl" doc:"The page the copy is on"`
	Version string `json:"version" doc:"publishedVersion, acceptedVersion, or submittedVersion; empty when not said"`
	License string `json:"license" doc:"As OpenAlex writes it (cc-by, cc0, public-domain); empty when none is given"`
	Host    string `json:"host" doc:"The repository or journal it is in"`
}

// OpenCopyByDOI asks OpenAlex afresh for the best open copy of a DOI; nil when it knows none.
func (s *Service) OpenCopyByDOI(ctx context.Context, doi string) (*OpenCopy, error) {
	body, err := s.openAlexWork(ctx, doi, true)
	if err != nil || len(body) == 0 {
		return nil, err
	}
	return ParseOpenCopy(body)
}

// ParseOpenCopy reads a work's best open location; nil when it has none.
func ParseOpenCopy(body []byte) (*OpenCopy, error) {
	var w struct {
		Best *struct {
			PDFURL  *string `json:"pdf_url"`
			PageURL *string `json:"landing_page_url"`
			License *string `json:"license"`
			Version *string `json:"version"`
			Source  *struct {
				DisplayName string `json:"display_name"`
			} `json:"source"`
		} `json:"best_oa_location"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("openalex answer: %w", err)
	}
	b := w.Best
	if b == nil {
		return nil, nil
	}
	text := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	c := &OpenCopy{PDFURL: text(b.PDFURL), PageURL: text(b.PageURL), Version: text(b.Version), License: text(b.License)}
	if b.Source != nil {
		c.Host = strings.TrimSpace(b.Source.DisplayName)
	}
	if c.PDFURL == "" && c.PageURL == "" {
		return nil, nil
	}
	return c, nil
}

// ParseOpenAlex reads a work's keywords and its main topic, as subjects. Its older "concepts"
// are left out: tagged by machine across the whole index, they give a chapter on causation
// "Domain (mathematical analysis)".
func ParseOpenAlex(body []byte) ([]string, error) {
	var w struct {
		Keywords []struct {
			DisplayName string  `json:"display_name"`
			Score       float64 `json:"score"`
		} `json:"keywords"`
		PrimaryTopic *struct {
			DisplayName string `json:"display_name"`
		} `json:"primary_topic"`
	}
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("openalex answer: %w", err)
	}
	out := []string{}
	for _, k := range w.Keywords {
		if k.Score >= minKeywordScore {
			out = appendUnique(out, firstUpper(clean(k.DisplayName)))
		}
	}
	if w.PrimaryTopic != nil {
		out = appendUnique(out, clean(w.PrimaryTopic.DisplayName))
	}
	return out, nil
}

// firstUpper writes a keyword as a subject heading begins ("folk science" → "Folk science").
func firstUpper(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + strings.TrimSpace(s[n:])
}
