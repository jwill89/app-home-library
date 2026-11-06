package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// HeadingsURL is the Library of Congress's linked-data service. Its subject authority is the
// list of headings the catalog uses, all in English; it asks for no key.
const HeadingsURL = "https://id.loc.gov"

// SourceHeadings names the subject authority's answers in the lookup cache.
const SourceHeadings = "lcsh"

// Headings checks subject words against the Library of Congress Subject Headings.
type Headings struct {
	BaseURL string
	fetch   *fetcher
}

type suggestions struct {
	Hits []struct {
		ALabel string `json:"aLabel"`
		VLabel string `json:"vLabel"`
	} `json:"hits"`
}

// englishSubjects keeps the subjects whose heading is one of the Library of Congress's, in
// its authorized form; subdivisions after "--" ride along. Open Library's subjects come from
// every edition of a work in every language ("Socialisme", "Revolutie", a Russian party's
// name romanized five ways), and the catalog's headings are English, so a subject it does
// not know is left out. When the authority cannot be asked, the rest are left out too.
func (s *Service) englishSubjects(ctx context.Context, subjects []string) []string {
	var out []string
	for _, subject := range subjects {
		parts := strings.Split(subject, "--")
		heading, err := s.heading(ctx, parts[0])
		if err != nil {
			s.reason(SourceHeadings, err)
			break
		}
		if heading == "" {
			continue
		}
		for _, sub := range parts[1:] {
			if sub = strings.TrimSpace(sub); sub != "" {
				heading += " -- " + sub
			}
		}
		out = appendUnique(out, heading)
	}
	return out
}

// heading returns the authorized heading that words are, or are a variant of, and "" for
// words that are no heading.
func (s *Service) heading(ctx context.Context, words string) (string, error) {
	key := normalHeading(words)
	if key == "" {
		return "", nil
	}
	body, err := s.cached(ctx, key, SourceHeadings, false, func(ctx context.Context) ([]byte, error) {
		return s.Headings.fetch.get(ctx, s.Headings.BaseURL+"/authorities/subjects/suggest2?searchtype=leftanchored&count=10&q="+url.QueryEscape(key))
	})
	if err != nil || body == nil {
		return "", err
	}
	var found suggestions
	if err := json.Unmarshal(body, &found); err != nil {
		return "", fmt.Errorf("subject headings: %w", err)
	}
	for _, h := range found.Hits {
		if normalHeading(h.ALabel) == key || normalHeading(h.VLabel) == key {
			return h.ALabel, nil
		}
	}
	return "", nil
}

// normalHeading is a heading as headings compare: lower case, single spaces, no final period.
func normalHeading(s string) string {
	return strings.TrimRight(strings.ToLower(strings.Join(strings.Fields(s), " ")), ".")
}
