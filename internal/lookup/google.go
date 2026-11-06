package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// GoogleURL is the Books API.
const GoogleURL = "https://www.googleapis.com/books/v1"

// Google asks Google Books. Without a key it is not asked at all: the API gives keyless
// callers no quota.
type Google struct {
	BaseURL string
	Key     string
	fetch   *fetcher
}

func (g *Google) volumes(ctx context.Context, v url.Values) ([]byte, error) {
	if g.Key == "" {
		return nil, ErrUnavailable
	}
	v.Set("key", g.Key)
	return g.fetch.get(ctx, g.BaseURL+"/volumes?"+v.Encode())
}

// FetchISBN returns the raw answer for one ISBN: the whole record of the volume that carries
// it, as a search's answer of one. A search's answer leaves out the measurements, which the
// whole record has for most books (10 of 12 the other sources had no height for, 2026-11-05).
func (g *Google) FetchISBN(ctx context.Context, isbn string) ([]byte, error) {
	body, err := g.volumes(ctx, url.Values{"q": {"isbn:" + isbn}, "maxResults": {"3"}})
	if err != nil || body == nil {
		return body, err
	}
	var res struct {
		Items []googleVolume `json:"items"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	i := slices.IndexFunc(res.Items, func(v googleVolume) bool { return v.isbn13() == isbn && v.ID != "" })
	if i < 0 {
		return body, nil // no volume carries it, which ParseGoogle reads as nothing
	}
	whole, err := g.fetch.get(ctx, g.BaseURL+"/volumes/"+url.PathEscape(res.Items[i].ID)+"?"+url.Values{"key": {g.Key}}.Encode())
	if err != nil || whole == nil {
		return whole, err
	}
	return fmt.Appendf(nil, `{"items":[%s]}`, whole), nil
}

type googleVolume struct {
	ID         string `json:"id"`
	VolumeInfo struct {
		Title       string   `json:"title"`
		Subtitle    string   `json:"subtitle"`
		Authors     []string `json:"authors"`
		Publisher   string   `json:"publisher"`
		Published   string   `json:"publishedDate"`
		Description string   `json:"description"`
		Pages       int      `json:"pageCount"`
		Categories  []string `json:"categories"`
		Language    string   `json:"language"`
		Identifiers []struct {
			Type       string `json:"type"`
			Identifier string `json:"identifier"`
		} `json:"industryIdentifiers"`
		ImageLinks struct {
			Thumbnail string `json:"thumbnail"`
		} `json:"imageLinks"`
		// Only in a volume's whole record, as "20.80 cm".
		Dimensions struct {
			Height    string `json:"height"`
			Width     string `json:"width"`
			Thickness string `json:"thickness"`
		} `json:"dimensions"`
	} `json:"volumeInfo"`
}

func (v googleVolume) isbn13() string {
	for _, id := range v.VolumeInfo.Identifiers {
		if n, ok := NormalizeISBN(id.Identifier); ok && (id.Type == "ISBN_13" || id.Type == "ISBN_10") {
			return n
		}
	}
	return ""
}

// ParseGoogle reads FetchISBN's answer. Google's ISBN search now and then returns a
// different book first, so only a volume carrying the ISBN is taken.
func ParseGoogle(body []byte, isbn string) (*Record, error) {
	var res struct {
		Items []googleVolume `json:"items"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	for _, item := range res.Items {
		if item.isbn13() != isbn {
			continue
		}
		v := item.VolumeInfo
		r := &Record{
			Title:       strings.TrimSpace(v.Title),
			Subtitle:    strings.TrimSpace(v.Subtitle),
			Publisher:   strings.TrimSpace(v.Publisher),
			Published:   NormalizeDate(v.Published),
			Description: PlainText(v.Description),
			Pages:       v.Pages,
			Language:    NormalizeLanguage(v.Language),
			HeightMM:    googleLength(v.Dimensions.Height),
			WidthMM:     googleLength(v.Dimensions.Width),
			ThicknessMM: googleLength(v.Dimensions.Thickness),
		}
		for _, a := range v.Authors {
			r.Contributors = append(r.Contributors, Contributor{Name: strings.TrimSpace(a), Role: "author"})
		}
		for _, c := range v.Categories {
			r.Subjects = appendUnique(r.Subjects, c)
		}
		// No covers: Google's image host answers the droplet's address with a "Sorry" page
		// (403) while serving the same image to a home connection, so a Google cover could be
		// shown but never kept (docs/decisions.md, 2026-11-04).
		return r, nil
	}
	return nil, nil
}

var googleLengthRe = regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*([a-z]+)`)

// googleLength reads one of Google's measurements ("20.80 cm") into millimetres; 0 when
// there is none, or none that makes sense for a book.
func googleLength(s string) int {
	m := googleLengthRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil || v <= 0 {
		return 0
	}
	if mm := lengthMM(v, m[2]); mm < 1000 {
		return mm
	}
	return 0
}

// thumbnail is a Google cover link a browser can show: over HTTPS, without the page-curl
// effect Google paints on by default.
func thumbnail(link string) string {
	u, err := url.Parse(strings.Replace(link, "http://", "https://", 1))
	if link == "" || err != nil {
		return ""
	}
	q := u.Query()
	q.Del("edge")
	u.RawQuery = q.Encode()
	return u.String()
}

// Search asks Google Books for books matching the words.
func (g *Google) Search(ctx context.Context, q string) ([]Candidate, error) {
	body, err := g.volumes(ctx, url.Values{"q": {q}, "maxResults": {"20"}, "printType": {"books"}})
	if err != nil || body == nil {
		return nil, err
	}
	var res struct {
		Items []googleVolume `json:"items"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("google search: %w", err)
	}
	var out []Candidate
	for _, item := range res.Items {
		v := item.VolumeInfo
		c := Candidate{
			Source:    SourceGoogle,
			Title:     v.Title,
			Subtitle:  v.Subtitle,
			Authors:   v.Authors,
			Publisher: v.Publisher,
			Published: NormalizeDate(v.Published),
			ISBN13:    item.isbn13(),
			Thumbnail: thumbnail(v.ImageLinks.Thumbnail),
		}
		out = append(out, c)
	}
	return out, nil
}
