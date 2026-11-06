package lookup

import "strings"

// Source names, which are also the values in an edition's field sources.
const (
	SourceLoC         = "loc"
	SourceOpenLibrary = "openlibrary"
	SourceGoogle      = "google"
	SourceLoCRelated  = "loc-related" // another edition of the same work in the LoC catalog
	SourceSeries      = "series"      // the number the rest of the series has
)

// Proposal is the merged book: one value for each field and the source each came from.
type Proposal struct {
	ISBN13           string            `json:"isbn13"`
	ISBN10           string            `json:"isbn10"`
	LCCN             string            `json:"lccn"`
	Title            string            `json:"title"`
	Subtitle         string            `json:"subtitle"`
	Contributors     []Contributor     `json:"contributors"`
	Publisher        string            `json:"publisher"`
	PublishPlace     string            `json:"publishPlace"`
	Published        string            `json:"published"`
	EditionStatement string            `json:"editionStatement"`
	Format           string            `json:"format" enum:",hardcover,paperback,trade_paperback,mass_market,library_binding,spiral,board_book,leather,other"`
	Pages            int               `json:"pages"`
	HeightMM         int               `json:"heightMm"`
	WidthMM          int               `json:"widthMm"`
	ThicknessMM      int               `json:"thicknessMm"`
	Language         string            `json:"language"`
	DDC              string            `json:"ddc"`
	LCC              string            `json:"lcc"`
	Subjects         []string          `json:"subjects"`
	Series           string            `json:"series"`
	SeriesNumber     string            `json:"seriesNumber"`
	Description      string            `json:"description"`
	Covers           []CoverChoice     `json:"covers"`
	Sources          map[string]string `json:"sources"`
}

// CoverChoice is a cover a source has for the book, and which source.
type CoverChoice struct {
	URL    string `json:"url"`
	Source string `json:"source" enum:"openlibrary,hardcover"`
}

// The order each field is taken in. Google and Open Library write titles, names, and
// publishers as the book does; the Library of Congress writes them as a catalogue does
// (sentence case, names inverted), but its classification and measurements come from
// a cataloguer with the book in hand. HathiTrust's records are the research libraries'
// cataloguers' too, so they come next after the Library of Congress' for those, and last for
// the rest. It names no series: what it gives as one is most often a publisher's line
// ("Bantam spectra book").
var (
	byText     = []string{SourceGoogle, SourceOpenLibrary, SourceLoC, SourceHathiTrust}
	byPages    = []string{SourceOpenLibrary, SourceGoogle, SourceLoC, SourceHathiTrust}
	byCatalog  = []string{SourceLoC, SourceHathiTrust, SourceOpenLibrary, SourceGoogle}
	byPhysical = []string{SourceOpenLibrary, SourceLoC, SourceHathiTrust, SourceGoogle}
	// Hardcover names a series only where no other source does.
	bySeries       = []string{SourceOpenLibrary, SourceLoC, SourceGoogle, SourceHardcover}
	bySeriesNumber = []string{SourceLoC, SourceOpenLibrary, SourceGoogle, SourceHardcover}
)

// Merge combines what each source said. A source that did not answer is absent from recs.
func Merge(isbn13 string, recs map[string]*Record) Proposal {
	p := Proposal{ISBN13: isbn13, ISBN10: ISBN10(isbn13), Sources: map[string]string{}}
	pick := func(field string, order []string, get func(*Record) string) string {
		for _, src := range order {
			if r := recs[src]; r != nil {
				if v := strings.TrimSpace(get(r)); v != "" {
					p.Sources[field] = src
					return v
				}
			}
		}
		return ""
	}
	pickInt := func(field string, order []string, get func(*Record) int) int {
		for _, src := range order {
			if r := recs[src]; r != nil && get(r) > 0 {
				p.Sources[field] = src
				return get(r)
			}
		}
		return 0
	}

	p.Title = pick("title", byText, func(r *Record) string { return r.Title })
	if src := p.Sources["title"]; src != "" {
		// The subtitle goes with the title it was written under.
		p.Subtitle = recs[src].Subtitle
		if p.Subtitle != "" {
			p.Sources["subtitle"] = src
		}
	}
	p.Publisher = pick("publisher", byText, func(r *Record) string { return r.Publisher })
	p.PublishPlace = pick("publishPlace", byCatalog, func(r *Record) string { return r.Place })
	p.Published = mergeDate(&p, recs)
	p.EditionStatement = pick("editionStatement", byCatalog, func(r *Record) string { return r.Edition })
	p.Format = mergeFormat(&p, recs)
	p.Pages = pickInt("pages", byPages, func(r *Record) int { return r.Pages })
	p.HeightMM = pickInt("heightMm", byCatalog, func(r *Record) int { return r.HeightMM })
	p.WidthMM = pickInt("widthMm", byPhysical, func(r *Record) int { return r.WidthMM })
	p.ThicknessMM = pickInt("thicknessMm", byPhysical, func(r *Record) int { return r.ThicknessMM })
	p.Language = pick("language", byText, func(r *Record) string { return r.Language })
	p.DDC = pick("ddc", byCatalog, func(r *Record) string { return r.DDC })
	p.LCC = pick("lcc", byCatalog, func(r *Record) string { return r.LCC })
	p.LCCN = pick("lccn", byCatalog, func(r *Record) string { return r.LCCN })
	p.Series = englishSeries(pick("series", bySeries, func(r *Record) string { return r.Series }))
	p.SeriesNumber = pick("seriesNumber", bySeriesNumber, func(r *Record) string { return r.SeriesNumber })
	p.Description = pick("description", byText, func(r *Record) string { return r.Description })
	p.Contributors = mergeContributors(&p, recs)
	p.Subjects = mergeSubjects(&p, recs)
	// Covers are Open Library's, then Hardcover's: the ones the server can fetch and keep.
	for _, src := range []string{SourceOpenLibrary, SourceHardcover} {
		if r := recs[src]; r != nil {
			for _, url := range r.Covers {
				p.Covers = append(p.Covers, CoverChoice{URL: url, Source: src})
			}
		}
	}
	return p
}

// mergeDate keeps the first source's date and takes a more precise one for the same year.
func mergeDate(p *Proposal, recs map[string]*Record) string {
	best, from := "", ""
	for _, src := range byText {
		r := recs[src]
		if r == nil || r.Published == "" {
			continue
		}
		switch {
		case best == "":
			best, from = r.Published, src
		case r.Published[:4] == best[:4] && len(r.Published) > len(best):
			best, from = r.Published, src
		}
	}
	if from != "" {
		p.Sources["published"] = from
	}
	return best
}

// mergeFormat takes the first binding named, and a more exact paperback when any source
// knows whether it is mass market or trade.
func mergeFormat(p *Proposal, recs map[string]*Record) string {
	format, from := "", ""
	for _, src := range byPhysical {
		if r := recs[src]; r != nil && r.Format != "" {
			format, from = r.Format, src
			break
		}
	}
	if format == "paperback" {
		for _, src := range byPhysical {
			if r := recs[src]; r != nil && (r.Format == "mass_market" || r.Format == "trade_paperback") {
				format, from = r.Format, src
				break
			}
		}
	}
	if from != "" {
		p.Sources["format"] = from
	}
	return format
}

// mergeContributors takes the authors from the first source that names any, then adds the
// translators, illustrators, and editors the catalog names that are not there yet.
func mergeContributors(p *Proposal, recs map[string]*Record) []Contributor {
	var out []Contributor
	for _, src := range byText {
		r := recs[src]
		if r == nil || len(r.Contributors) == 0 {
			continue
		}
		out = append(out, r.Contributors...)
		p.Sources["contributors"] = src
		break
	}
	have := map[string]bool{}
	for _, c := range out {
		have[strings.ToLower(c.Name)] = true
	}
	for _, src := range byCatalog {
		r := recs[src]
		if r == nil {
			continue
		}
		for _, c := range r.Contributors {
			if c.Role == "author" || c.Role == "contributor" || have[strings.ToLower(c.Name)] || c.Name == "" {
				continue
			}
			have[strings.ToLower(c.Name)] = true
			out = append(out, c)
		}
	}
	return out
}

// mergeSubjects joins the catalog's subject headings and Google's categories: the Library of
// Congress' headings, or HathiTrust's where it has none (the same headings, written alike, so
// both would say each twice). Open Library's subjects are user-entered, noisy, and in every
// language, so they are used only when none of those has any, and marked as Open Library's for
// the lookup to keep only the English headings.
func mergeSubjects(p *Proposal, recs map[string]*Record) []string {
	var out []string
	catalog := SourceLoC
	if r := recs[SourceLoC]; r == nil || len(r.Subjects) == 0 {
		catalog = SourceHathiTrust
	}
	for _, src := range []string{catalog, SourceGoogle} {
		if r := recs[src]; r != nil {
			for _, s := range r.Subjects {
				out = appendUnique(out, s)
			}
		}
	}
	if len(out) > 0 {
		p.Sources["subjects"] = "merged"
	} else if r := recs[SourceOpenLibrary]; r != nil && len(r.Subjects) > 0 {
		out = append(out, r.Subjects...)
		p.Sources["subjects"] = SourceOpenLibrary
	}
	if len(out) > 15 {
		out = out[:15]
	}
	return out
}
