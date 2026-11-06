package lookup

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"slices"
)

// HathiTrustURL is HathiTrust's Bibliographic API: the research libraries' shared catalog, as
// MARC, free and without a key. Its metadata is CC0.
const HathiTrustURL = "https://catalog.hathitrust.org/api/volumes"

// SourceHathiTrust names HathiTrust's answers.
const SourceHathiTrust = "hathitrust"

// HathiTrust asks HathiTrust's catalog: academic and older books the Library of Congress
// lacks or has no class number for.
type HathiTrust struct {
	BaseURL string
	fetch   *fetcher
}

// FetchISBN returns the raw answer for one ISBN: the full records, each with its MARC.
func (h *HathiTrust) FetchISBN(ctx context.Context, isbn string) ([]byte, error) {
	return h.fetch.get(ctx, h.BaseURL+"/full/isbn/"+isbn+".json")
}

// ParseHathiTrust reads FetchISBN's answer: the first record that lists the ISBN, read as the
// Library of Congress' MARC is. nil when there is none.
func ParseHathiTrust(body []byte, isbn string) (*Record, error) {
	var res struct {
		Records map[string]struct {
			ISBNs []string `json:"isbns"`
			MARC  string   `json:"marc-xml"`
		} `json:"records"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("hathitrust: %w", err)
	}
	// The map has no order; the lowest record number is the same answer every time.
	ids := make([]string, 0, len(res.Records))
	for id := range res.Records {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		rec := res.Records[id]
		if !slices.ContainsFunc(rec.ISBNs, func(s string) bool { n, ok := NormalizeISBN(s); return ok && n == isbn }) {
			continue
		}
		var c struct {
			Records []marcRecord `xml:"record"`
		}
		if err := xml.Unmarshal([]byte(rec.MARC), &c); err != nil {
			return nil, fmt.Errorf("hathitrust %s: %w", id, err)
		}
		if len(c.Records) > 0 {
			return c.Records[0].record(isbn), nil
		}
	}
	return nil, nil
}
