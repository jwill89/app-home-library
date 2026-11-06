package catalog

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"homelibrary/internal/db"
)

// Shelving rules: what a section's call numbers are made of.
const (
	ShelveByDewey  = "dewey"  // 823.92 / MCD
	ShelveByAuthor = "author" // FIC / MCD
	ShelveBySeries = "series" // MANGA / ONE / v.12
)

// CallNumber is a copy's call number as a spine label stacks it: the class (a Dewey number or
// the section's prefix), the Cutter, and the volume for a book in a series. Lines that would
// be empty are left out.
func CallNumber(s *db.CopyShelving) []string {
	class, cutter := callParts(s)
	lines := []string{}
	for _, l := range []string{class, cutter, volume(s.SeriesNumber)} {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func callParts(s *db.CopyShelving) (class, cutter string) {
	class, cutter = s.Ddc, cutterOf(s.AuthorSort, s.Title)
	switch s.Shelving {
	case ShelveByAuthor:
		class = s.LabelPrefix
	case ShelveBySeries:
		class = s.LabelPrefix
		if s.Series != "" {
			cutter = cutterOf("", s.Series)
		}
	default:
		if class == "" {
			class = s.LabelPrefix
		}
	}
	return class, cutter
}

// cutterOf is the three-letter Cutter: the first letters of the author's surname (the sort
// name begins with it), or of the title, past a leading article, for a book with no author.
func cutterOf(authorSort, title string) string {
	name := authorSort
	if name == "" {
		name = withoutArticle(title)
	}
	var b strings.Builder
	for _, r := range name {
		if b.Len() == 3 {
			break
		}
		if r = fold(r); r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// withoutArticle is a title or a series past a leading article, as it is filed: "The Hobbit"
// under Hobbit.
func withoutArticle(title string) string {
	t := strings.TrimSpace(title)
	if first, rest, ok := strings.Cut(t, " "); ok && rest != "" && isArticle(first) {
		return rest
	}
	return t
}

// isArticle is whether a word is one a title or a series is filed past.
func isArticle(word string) bool {
	switch strings.ToLower(word) {
	case "the", "a", "an":
		return true
	}
	return false
}

// keyWords are a name's words as two ways of writing it are matched: lower case, letters and
// digits only.
func keyWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// fold makes a letter a plain capital: Ä is A, ø is O. What is not a Latin letter is left as
// it is, and the Cutter skips it.
func fold(r rune) rune {
	r = unicode.ToUpper(r)
	if r < 0x80 {
		return r
	}
	for plain, accented := range accents {
		if strings.ContainsRune(accented, r) {
			return plain
		}
	}
	return r
}

var accents = map[rune]string{
	'A': "ÀÁÂÃÄÅĀĂĄǍ", 'C': "ÇĆĈĊČ", 'D': "ĎĐ", 'E': "ÈÉÊËĒĔĖĘĚ", 'G': "ĜĞĠĢ", 'H': "ĤĦ",
	'I': "ÌÍÎÏĨĪĬĮİ", 'J': "Ĵ", 'K': "Ķ", 'L': "ĹĻĽĿŁ", 'N': "ÑŃŅŇ", 'O': "ÒÓÔÕÖØŌŎŐ",
	'R': "ŔŖŘ", 'S': "ŚŜŞŠ", 'T': "ŢŤŦ", 'U': "ÙÚÛÜŨŪŬŮŰŲ", 'W': "Ŵ", 'Y': "ÝŶŸ", 'Z': "ŹŻŽ",
}

func volume(n string) string {
	if n = strings.TrimSpace(n); n == "" {
		return ""
	}
	return "v." + n
}

// shelfOrder is where a copy stands among the others: by section, class, and Cutter, then an
// author's series kept together in volume order, then by title. It is worked out once a copy,
// since a sort or a layout compares each copy many times, and each comparison had parsed both
// call numbers again.
type shelfOrder struct {
	section       string
	class, number line
	cutter        string
	series, title string
	copyID        int64
}

func orderOf(s *db.CopyShelving) shelfOrder {
	class, cutter := callParts(s)
	return shelfOrder{
		section: s.Section, class: lineOf(class), cutter: cutter,
		series: strings.ToLower(withoutArticle(s.Series)), number: lineOf(s.SeriesNumber),
		title: strings.ToLower(withoutArticle(s.Title)), copyID: s.CopyID,
	}
}

func (a shelfOrder) compare(b shelfOrder) int {
	return cmp.Or(
		cmp.Compare(a.section, b.section),
		a.class.compare(b.class),
		cmp.Compare(a.cutter, b.cutter),
		cmp.Compare(a.series, b.series),
		a.number.compare(b.number),
		cmp.Compare(a.title, b.title),
		cmp.Compare(a.copyID, b.copyID),
	)
}

// sortShelved puts things in the order their copies stand on the shelf.
func sortShelved[T any](items []T, shelving func(*T) *db.CopyShelving) {
	type placed struct {
		item  T
		order shelfOrder
	}
	all := make([]placed, len(items))
	for i := range items {
		all[i] = placed{items[i], orderOf(shelving(&items[i]))}
	}
	slices.SortStableFunc(all, func(a, b placed) int { return a.order.compare(b.order) })
	for i := range all {
		items[i] = all[i].item
	}
}

// line is a call number's line as it sorts: numbers as numbers, 153.42 before 823.92 and v.2
// before v.10.
type line struct {
	text  string
	num   float64
	isNum bool
}

func lineOf(text string) line {
	n, err := strconv.ParseFloat(strings.TrimPrefix(text, "v."), 64)
	return line{text: text, num: n, isNum: err == nil}
}

func (a line) compare(b line) int {
	switch {
	case a.isNum && b.isNum:
		return cmp.Compare(a.num, b.num)
	case a.text == "" || b.text == "":
		return cmp.Compare(a.text, b.text) // nothing before something
	case a.isNum:
		return -1 // a class number before a prefix
	case b.isNum:
		return 1
	}
	return cmp.Compare(a.text, b.text)
}
