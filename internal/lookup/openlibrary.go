package lookup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// OpenLibraryURL is Open Library's site, which serves its API.
const OpenLibraryURL = "https://openlibrary.org"

// OpenLibrary asks Open Library. Its books API answers in two shapes: "data" has the author
// names and the cover, "details" has the edition record itself with the binding, the
// dimensions, and the series. One lookup asks for both.
type OpenLibrary struct {
	BaseURL   string
	CoversURL string
	fetch     *fetcher
}

// FetchISBN returns both answers for one ISBN as one JSON object, or nil if Open Library
// does not know the book.
func (o *OpenLibrary) FetchISBN(ctx context.Context, isbn string) ([]byte, error) {
	base := o.BaseURL + "/api/books?format=json&bibkeys=ISBN:" + isbn + "&jscmd="
	data, err := o.fetch.get(ctx, base+"data")
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || string(data) == "{}" {
		return nil, nil
	}
	details, err := o.fetch.get(ctx, base+"details")
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]json.RawMessage{"data": data, "details": orEmpty(details)})
}

func orEmpty(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return b
}

type olData struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Authors  []struct {
		Name string `json:"name"`
	} `json:"authors"`
	Publishers []struct {
		Name string `json:"name"`
	} `json:"publishers"`
	PublishPlaces []struct {
		Name string `json:"name"`
	} `json:"publish_places"`
	PublishDate     string `json:"publish_date"`
	NumberOfPages   int    `json:"number_of_pages"`
	Identifiers     map[string][]string
	Classifications struct {
		DDC []string `json:"dewey_decimal_class"`
		LCC []string `json:"lc_classifications"`
	} `json:"classifications"`
	Subjects []struct {
		Name string `json:"name"`
	} `json:"subjects"`
	Cover struct {
		Large  string `json:"large"`
		Medium string `json:"medium"`
	} `json:"cover"`
}

type olDetails struct {
	Details struct {
		PhysicalFormat     string   `json:"physical_format"`
		PhysicalDimensions string   `json:"physical_dimensions"`
		Series             []string `json:"series"`
		DDC                []string `json:"dewey_decimal_class"`
		LCC                []string `json:"lc_classifications"`
		EditionName        string   `json:"edition_name"`
		Languages          []struct {
			Key string `json:"key"`
		} `json:"languages"`
		Covers      []int64         `json:"covers"`
		Description json.RawMessage `json:"description"`
		Contributor []struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"contributors"`
	} `json:"details"`
}

// ParseOpenLibrary reads what FetchISBN returned.
func ParseOpenLibrary(body []byte, coversURL string) (*Record, error) {
	var both struct {
		Data    map[string]olData    `json:"data"`
		Details map[string]olDetails `json:"details"`
	}
	if err := json.Unmarshal(body, &both); err != nil {
		return nil, fmt.Errorf("open library: %w", err)
	}
	var d olData
	for _, v := range both.Data {
		d = v
	}
	var x olDetails
	for _, v := range both.Details {
		x = v
	}
	r := &Record{
		Title:     strings.TrimSpace(d.Title),
		Subtitle:  strings.TrimSpace(d.Subtitle),
		Published: NormalizeDate(d.PublishDate),
		Pages:     d.NumberOfPages,
		Edition:   cleanField(x.Details.EditionName),
		Format:    FormatFromText(x.Details.PhysicalFormat),
	}
	for _, a := range d.Authors {
		r.Contributors = append(r.Contributors, Contributor{Name: strings.TrimSpace(a.Name), Role: "author"})
	}
	for _, c := range x.Details.Contributor {
		r.Contributors = append(r.Contributors, Contributor{Name: strings.TrimSpace(c.Name), Role: marcRole(c.Role)})
	}
	if len(d.Publishers) > 0 {
		r.Publisher = cleanField(d.Publishers[0].Name)
	}
	if len(d.PublishPlaces) > 0 {
		r.Place = cleanField(d.PublishPlaces[0].Name)
	}
	for _, c := range slices.Concat(d.Classifications.DDC, x.Details.DDC) {
		if n := NormalizeDDC(c); n != "" {
			r.DDC = n
			break
		}
	}
	lcc := slices.Concat(d.Classifications.LCC, x.Details.LCC)
	if len(lcc) > 0 {
		r.LCC = strings.TrimSpace(lcc[0])
	}
	if v := d.Identifiers["lccn"]; len(v) > 0 {
		r.LCCN = v[0]
	}
	for i, s := range d.Subjects {
		if i == 8 {
			break
		}
		r.Subjects = appendUnique(r.Subjects, s.Name)
	}
	r.HeightMM, r.WidthMM, r.ThicknessMM = Dimensions(x.Details.PhysicalDimensions)
	if len(x.Details.Series) > 0 {
		r.Series, r.SeriesNumber = SplitSeries(x.Details.Series[0])
	}
	if len(x.Details.Languages) > 0 {
		r.Language = NormalizeLanguage(x.Details.Languages[0].Key)
	}
	r.Description = PlainText(olText(x.Details.Description))
	// Every picture of the edition, each as it was uploaded: Open Library's "large" is cut to
	// 500 pixels high, and many uploads are three times that.
	for _, id := range x.Details.Covers {
		if id > 0 {
			r.Covers = appendUnique(r.Covers, coversURL+"/b/id/"+strconv.FormatInt(id, 10)+".jpg")
		}
	}
	if big, ok := strings.CutSuffix(d.Cover.Large, "-L.jpg"); ok && len(r.Covers) == 0 {
		r.Covers = append(r.Covers, big+".jpg")
	}
	return r, nil
}

// olText reads a field Open Library writes either as a string or as {"value": "..."}.
func olText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s)
	}
	var v struct {
		Value string `json:"value"`
	}
	_ = json.Unmarshal(raw, &v) // a shape it does not know reads as no description
	return strings.TrimSpace(v.Value)
}

// Search asks Open Library's search for books matching the words, one edition each.
func (o *OpenLibrary) Search(ctx context.Context, q string) ([]Candidate, error) {
	v := url.Values{
		"q":      {q},
		"limit":  {"20"},
		"fields": {"title,subtitle,author_name,first_publish_year,cover_i,editions,editions.title,editions.subtitle,editions.isbn,editions.publisher,editions.publish_date,editions.cover_i"},
	}
	body, err := o.fetch.get(ctx, o.BaseURL+"/search.json?"+v.Encode())
	if err != nil || body == nil {
		return nil, err
	}
	return parseOpenLibrarySearch(body, o.CoversURL)
}

func parseOpenLibrarySearch(body []byte, coversURL string) ([]Candidate, error) {
	var res struct {
		Docs []struct {
			Title    string   `json:"title"`
			Subtitle string   `json:"subtitle"`
			Authors  []string `json:"author_name"`
			Year     int      `json:"first_publish_year"`
			CoverID  int64    `json:"cover_i"`
			Editions struct {
				Docs []struct {
					Title       string   `json:"title"`
					Subtitle    string   `json:"subtitle"`
					ISBN        []string `json:"isbn"`
					Publisher   []string `json:"publisher"`
					PublishDate []string `json:"publish_date"`
					CoverID     int64    `json:"cover_i"`
				} `json:"docs"`
			} `json:"editions"`
		} `json:"docs"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("open library search: %w", err)
	}
	var out []Candidate
	for _, d := range res.Docs {
		c := Candidate{Source: SourceOpenLibrary, Title: d.Title, Subtitle: d.Subtitle, Authors: d.Authors}
		if d.Year > 0 {
			c.Published = strconv.Itoa(d.Year)
		}
		cover := d.CoverID
		if len(d.Editions.Docs) > 0 {
			e := d.Editions.Docs[0]
			if e.Title != "" {
				c.Title, c.Subtitle = e.Title, e.Subtitle
			}
			for _, i := range e.ISBN {
				if n, ok := NormalizeISBN(i); ok {
					c.ISBN13 = n
					if len(i) == 13 {
						break
					}
				}
			}
			if len(e.Publisher) > 0 {
				c.Publisher = e.Publisher[0]
			}
			if len(e.PublishDate) > 0 {
				c.Published = NormalizeDate(e.PublishDate[0])
			}
			if e.CoverID > 0 {
				cover = e.CoverID
			}
		}
		if cover > 0 {
			id := coversURL + "/b/id/" + strconv.FormatInt(cover, 10)
			c.Thumbnail = id + "-M.jpg"
			c.Covers = []CoverChoice{{URL: id + ".jpg", Source: SourceOpenLibrary}}
		}
		out = append(out, c)
	}
	return out, nil
}
