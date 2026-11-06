package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// HardcoverURL is Hardcover's GraphQL API. It is free with an account's token, asks for no
// more than 60 requests a minute, and is asked only when the token is set.
const HardcoverURL = "https://api.hardcover.app/v1/graphql"

// SourceHardcover names Hardcover's answers.
const SourceHardcover = "hardcover"

// Hardcover asks Hardcover about an ISBN, for the covers and series the other sources lack.
type Hardcover struct {
	URL   string
	Token string
	fetch *fetcher
}

const hardcoverQuery = `query($isbn: String!) { editions(where: {isbn_13: {_eq: $isbn}}, limit: 3) { image { url } book { image { url } book_series(order_by: {position: asc}, limit: 1) { position series { name } } } } }`

// FetchISBN returns Hardcover's answer for an ISBN-13.
func (h *Hardcover) FetchISBN(ctx context.Context, isbn13 string) ([]byte, error) {
	body, err := json.Marshal(map[string]any{"query": hardcoverQuery, "variables": map[string]string{"isbn": isbn13}})
	if err != nil {
		return nil, err
	}
	return h.fetch.post(ctx, h.URL, body, "Bearer "+h.Token)
}

type hardcoverAnswer struct {
	Data struct {
		Editions []struct {
			Image *struct {
				URL string `json:"url"`
			} `json:"image"`
			Book struct {
				Image *struct {
					URL string `json:"url"`
				} `json:"image"`
				BookSeries []struct {
					Position *float64 `json:"position"`
					Series   struct {
						Name string `json:"name"`
					} `json:"series"`
				} `json:"book_series"`
			} `json:"book"`
		} `json:"editions"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// ParseHardcover reads Hardcover's answer: the edition's cover, else the book's, and its
// series. A series named in another script is left out; the library keeps English names. It
// returns nil when Hardcover does not know the ISBN.
func ParseHardcover(body []byte) (*Record, error) {
	var a hardcoverAnswer
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("hardcover: %w", err)
	}
	if len(a.Errors) > 0 {
		return nil, fmt.Errorf("hardcover: %s", a.Errors[0].Message)
	}
	if len(a.Data.Editions) == 0 {
		return nil, nil
	}
	r := &Record{}
	for _, e := range a.Data.Editions {
		if e.Image != nil && e.Image.URL != "" {
			r.Covers = appendUnique(r.Covers, e.Image.URL)
		}
	}
	for _, e := range a.Data.Editions {
		if e.Book.Image != nil && e.Book.Image.URL != "" {
			r.Covers = appendUnique(r.Covers, e.Book.Image.URL)
		}
		if bs := e.Book.BookSeries; r.Series == "" && len(bs) > 0 && latin(bs[0].Series.Name) {
			r.Series = cleanField(bs[0].Series.Name)
			if p := bs[0].Position; p != nil && *p > 0 {
				r.SeriesNumber = strconv.FormatFloat(*p, 'f', -1, 64)
			}
		}
	}
	return r, nil
}

// latin reports whether every letter in s is in the Latin script.
func latin(s string) bool {
	for _, c := range s {
		if unicode.IsLetter(c) && !unicode.Is(unicode.Latin, c) {
			return false
		}
	}
	return true
}

// englishSeries keeps the Latin half of a series named in two scripts, as Open Library and
// Hardcover name translated manga: "ヲタクに恋は難しい / Wotakoi: Love is Hard for Otaku".
func englishSeries(s string) string {
	parts := strings.Split(s, " / ")
	if len(parts) < 2 {
		return s
	}
	var keep []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" && latin(p) {
			keep = append(keep, p)
		}
	}
	if len(keep) == 1 {
		return keep[0]
	}
	return s
}
