// Package lookup asks the Library of Congress, HathiTrust, Open Library, Google Books, and
// Hardcover about a book and merges what they say, field by field, into one proposal.
package lookup

import (
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Record is what one source says about one book, already in the catalog's terms.
type Record struct {
	Title        string
	Subtitle     string
	Contributors []Contributor
	Publisher    string
	Place        string
	Published    string // YYYY, YYYY-MM, or YYYY-MM-DD
	Edition      string
	Format       string // one of the editions.format values, or empty
	Pages        int
	HeightMM     int
	WidthMM      int
	ThicknessMM  int
	Language     string // ISO 639-1 where known
	DDC          string
	LCC          string
	LCCN         string
	Subjects     []string
	Series       string
	SeriesNumber string
	Description  string
	Covers       []string // image URLs, best first
}

// Contributor is a person and what they did for the book.
type Contributor struct {
	Name string `json:"name"`
	Role string `json:"role" enum:"author,editor,translator,illustrator,narrator,contributor"`
}

// Roles a contributor may have, in the order they are shown.
var Roles = []string{"author", "editor", "translator", "illustrator", "narrator", "contributor"}

// Formats an edition may have.
var Formats = []string{"hardcover", "paperback", "trade_paperback", "mass_market", "library_binding", "spiral", "board_book", "leather", "other"}

// FormatFromText reads a binding as sources and other apps write it ("Mass Market Paperback",
// "pbk.", "hardcover : acid-free paper") and returns the catalog's word, or empty. Paper
// quality ("alk. paper") is not a binding, so a bare "paper" is never read as one.
func FormatFromText(s string) string {
	t := strings.ToLower(s)
	switch {
	case t == "":
		return ""
	case strings.Contains(t, "ebook"), strings.Contains(t, "e-book"), strings.Contains(t, "electronic"), strings.Contains(t, "audio"):
		return ""
	case strings.Contains(t, "mass market"), strings.Contains(t, "mass-market"):
		return "mass_market"
	case strings.Contains(t, "trade") && (strings.Contains(t, "pap") || strings.Contains(t, "pbk")):
		return "trade_paperback"
	case strings.Contains(t, "lib. bdg"), strings.Contains(t, "library binding"), strings.Contains(t, "library bound"):
		return "library_binding"
	case strings.Contains(t, "board book"), strings.Contains(t, "bd. bk"):
		return "board_book"
	case strings.Contains(t, "spiral"), strings.Contains(t, "comb bound"), strings.Contains(t, "wire-o"):
		return "spiral"
	case strings.Contains(t, "leather"):
		return "leather"
	case strings.Contains(t, "hardcover"), strings.Contains(t, "hardback"), strings.Contains(t, "hardbound"),
		strings.Contains(t, "hbk"), strings.Contains(t, "cloth"), wordIn(t, "hc"), wordIn(t, "hc."):
		return "hardcover"
	case strings.Contains(t, "pbk"), strings.Contains(t, "paperback"), strings.Contains(t, "softcover"),
		strings.Contains(t, "soft cover"), strings.Contains(t, "paperbound"), wordIn(t, "pb"):
		return "paperback"
	}
	return ""
}

func wordIn(s, word string) bool {
	return slices.Contains(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == ';' || r == ':' || r == ','
	}), word)
}

var ddcShape = regexp.MustCompile(`^\d{3}(\.\d+)?$`)

// NormalizeDDC turns a Dewey number as catalogues write it ("153.4/2", "823/.92") into the
// number itself, and drops what is not one: "[Fic]", "B", "[E]".
func NormalizeDDC(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("/", "", "'", "", " ", "").Replace(s)
	s = strings.TrimSuffix(s, ".")
	if !ddcShape.MatchString(s) {
		return ""
	}
	return s
}

// yearRe finds a year in a field, one run into a letter too: "c1998" is copyright 1998.
var yearRe = regexp.MustCompile(`(?:^|\D)(1[4-9]\d\d|20\d\d|2100)(?:\D|$)`)

// Year is the first year in a field, or empty: "c1998", "[1985]", and "Ace (2014)" each have
// one.
func Year(s string) string {
	if m := yearRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

var dateLayouts = []struct {
	layout string
	out    string
}{
	{"2006-01-02", "2006-01-02"},
	{"2006-01", "2006-01"},
	{"January 2, 2006", "2006-01-02"},
	{"Jan 2, 2006", "2006-01-02"},
	{"January 02, 2006", "2006-01-02"},
	{"2 January 2006", "2006-01-02"},
	{"January 2006", "2006-01"},
	{"Jan 2006", "2006-01"},
}

// NormalizeDate reads a publication date in the shapes sources use and returns it as
// precisely as it was given: YYYY-MM-DD, YYYY-MM, or YYYY. Anything else gives its year.
func NormalizeDate(s string) string {
	s = strings.TrimSpace(strings.Trim(s, "[]©c. "))
	if s == "" {
		return ""
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l.layout, s); err == nil {
			return t.Format(l.out)
		}
	}
	return Year(s)
}

// InvertName turns a catalogue heading ("Kahneman, Daniel, 1934-") into the name as it is
// read ("Daniel Kahneman"). A heading without a comma is left as it is.
func InvertName(s string) string {
	s = trimHeadingStop(s)
	last, first, ok := strings.Cut(s, ",")
	if !ok {
		return s
	}
	first = strings.TrimSpace(first)
	// A suffix after a second comma ("King, Martin Luther, Jr.") follows the whole name.
	suffix := ""
	if f, sfx, ok := strings.Cut(first, ","); ok {
		first, suffix = strings.TrimSpace(f), strings.TrimSpace(sfx)
		if yearRe.MatchString(suffix) || strings.HasSuffix(suffix, "-") {
			suffix = ""
		}
	}
	name := strings.TrimSpace(first + " " + strings.TrimSpace(last))
	if suffix != "" {
		name += ", " + suffix
	}
	return name
}

// trimHeadingStop drops the comma or full stop a catalogue heading ends with, and keeps the
// stop of an initial or an abbreviation ("J. R. R.", "Jr.").
func trimHeadingStop(s string) string {
	s = strings.TrimRight(strings.TrimSpace(s), ", ")
	if !strings.HasSuffix(s, ".") {
		return s
	}
	words := strings.Fields(s)
	last := strings.TrimSuffix(words[len(words)-1], ".")
	if len([]rune(last)) <= 2 {
		return s
	}
	return strings.TrimSuffix(s, ".")
}

// Surname is the name a person is filed under, SortName's first part: "King" for "Martin
// Luther King, Jr.", and "Hume" for "Hume, David" as for "David Hume".
func Surname(name string) string {
	first, _, _ := strings.Cut(SortName(name), ", ")
	return first
}

// SortName is the name as a shelf sorts it: the last word first.
func SortName(name string) string {
	name = strings.TrimSpace(name)
	base, suffix, _ := strings.Cut(name, ", ")
	words := strings.Fields(base)
	if len(words) < 2 {
		return name
	}
	sorted := words[len(words)-1] + ", " + strings.Join(words[:len(words)-1], " ")
	if suffix != "" {
		sorted += ", " + suffix
	}
	return sorted
}

// cleanField trims the punctuation catalogues leave at the end of a field (" /", " :", ",").
func cleanField(s string) string {
	s = strings.TrimSpace(s)
	for {
		t := strings.TrimSpace(strings.TrimRight(s, "/:;,=."))
		t = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(t, "["), "]"))
		if t == s {
			return s
		}
		s = t
	}
}

var numberRe = regexp.MustCompile(`\d+(\.\d+)?`)

var numberWords = map[string]string{
	"one": "1", "two": "2", "three": "3", "four": "4", "five": "5", "six": "6", "seven": "7",
	"eight": "8", "nine": "9", "ten": "10", "eleven": "11", "twelve": "12", "thirteen": "13",
	"fourteen": "14", "fifteen": "15", "sixteen": "16", "seventeen": "17", "eighteen": "18",
	"nineteen": "19", "twenty": "20",
}

// readNumber finds a volume's number in what a series statement says of it ("v. 4", "bk. 9",
// "book one"): the figures, or a number written as a word.
func readNumber(s string) string {
	if n := numberRe.FindString(s); n != "" {
		return n
	}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) }) {
		if n, ok := numberWords[w]; ok {
			return n
		}
	}
	return ""
}

// "Book six of the Malazan book of the fallen": the number before the name.
var numberFirstRe = regexp.MustCompile(`(?i)^(?:book|bk\.|volume|vol\.)\s+(\S+)\s+of\s+(?:the\s+)?(.+)$`)

// SplitSeries reads a series statement into its name and number, as the sources write one:
// "Raven's Shadow, Book 1", "Raven's mark ; 3", "Wotakoi ; v. 4", "Malazan book of the fallen
// -- bk. 9", "The Witcher: Book 0.5", "Book six of the Malazan book of the fallen".
func SplitSeries(s string) (name, number string) {
	s = strings.TrimSpace(s)
	if m := numberFirstRe.FindStringSubmatch(s); m != nil {
		if n := readNumber(m[1]); n != "" {
			return cleanField(m[2]), n
		}
	}
	for _, sep := range []string{";", " -- ", ", Book ", ", book ", ": Book ", ": book ", ", Vol. ", ", vol. ", ": Vol. ", ", Volume ", ": Volume ", ", #", " #", ", v. ", " v. ", ", no. "} {
		if before, after, ok := strings.Cut(s, sep); ok {
			if n := readNumber(after); n != "" {
				return cleanField(before), n
			}
		}
	}
	if open := strings.LastIndex(s, "("); open > 0 && strings.HasSuffix(s, ")") {
		if n := numberRe.FindString(s[open:]); n != "" {
			return cleanField(s[:open]), n
		}
	}
	return cleanField(s), ""
}

var iso6392 = map[string]string{
	"eng": "en", "jpn": "ja", "fre": "fr", "fra": "fr", "ger": "de", "deu": "de", "spa": "es",
	"ita": "it", "por": "pt", "rus": "ru", "chi": "zh", "zho": "zh", "kor": "ko", "lat": "la",
	"grc": "grc", "gre": "el", "ell": "el", "heb": "he", "ara": "ar", "dut": "nl", "nld": "nl",
	"swe": "sv", "nor": "no", "dan": "da", "pol": "pl",
}

// NormalizeLanguage turns "eng", "/languages/eng", "English", or "en" into a two-letter code.
func NormalizeLanguage(s string) string {
	s = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "/languages/")))
	switch s {
	case "english":
		return "en"
	case "japanese":
		return "ja"
	}
	if v, ok := iso6392[s]; ok {
		return v
	}
	if len(s) == 2 {
		return s
	}
	return ""
}

// lengthMM reads a length with its unit ("24 cm", "7.9 inches") into millimetres.
func lengthMM(value float64, unit string) int {
	switch u := strings.ToLower(unit); {
	case strings.HasPrefix(u, "in"):
		return int(value*25.4 + 0.5)
	case strings.HasPrefix(u, "mm"), strings.HasPrefix(u, "milli"):
		return int(value + 0.5)
	default:
		return int(value*10 + 0.5)
	}
}

// Dimensions reads Open Library's "7.9 x 5.2 x 0.5 inches" into height, width, and
// thickness. Sources disagree on the order, so the longest side is the height and the
// shortest the thickness.
func Dimensions(s string) (height, width, thickness int) {
	nums := numberRe.FindAllString(s, -1)
	unit := "cm"
	if f := strings.Fields(s); len(f) > 0 {
		unit = f[len(f)-1]
	}
	var mm []int
	for _, n := range nums {
		v, err := strconv.ParseFloat(n, 64)
		if err != nil || v <= 0 {
			continue
		}
		mm = append(mm, lengthMM(v, unit))
	}
	slices.Sort(mm)
	slices.Reverse(mm)
	switch len(mm) {
	case 0:
		return 0, 0, 0
	case 1:
		return mm[0], 0, 0
	case 2:
		return mm[0], mm[1], 0
	}
	return mm[0], mm[1], mm[2]
}

var (
	tagRe       = regexp.MustCompile(`<[^>]+>`)
	paragraphRe = regexp.MustCompile(`(?i)<\s*/\s*(p|h[1-6])\s*>`)
	lineRe      = regexp.MustCompile(`(?i)<\s*(br|li|/div|/tr)\b[^>]*>`)
	blankRe     = regexp.MustCompile(`\n{3,}`)
)

// PlainText turns a description's markup into the text it shows: a paragraph's end as a
// blank line, a line break or a list item as a new line, every other tag dropped, entities
// decoded. The Bookshelf app's export writes < and > as &&L and &&R, so those are tags too.
// Plain text comes back as it went in.
func PlainText(s string) string {
	s = strings.NewReplacer("&&L", "<", "&&R", ">").Replace(s)
	s = paragraphRe.ReplaceAllString(s, "\n\n")
	s = lineRe.ReplaceAllString(s, "\n")
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(html.UnescapeString(s), " ", " ")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(blankRe.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
