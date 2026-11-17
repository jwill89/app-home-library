package catalog

import (
	"fmt"
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

// shelfKey is where a copy stands among the others, as text that sorts as the copies stand:
// by section, class, and Cutter, then an author's series kept together in volume order, then by
// title. Text, so the library list can carry it and a page order books the way the shelves do
// with a plain comparison; and worked out once a copy, since a sort or a layout compares each
// copy many times. A field never holds the separator, so a shorter one sorts first.
func shelfKey(s *db.CopyShelving) string {
	class, cutter := callParts(s)
	return strings.Join([]string{
		s.Section,
		classKey(class),
		cutter,
		strings.ToLower(withoutArticle(s.Series)),
		volumeKey(s.SeriesNumber),
		strings.ToLower(withoutArticle(s.Title)),
		fmt.Sprintf("%020d", s.CopyID),
	}, keySep)
}

// keySep comes before every letter, digit, and space, so it ends a field before any longer one.
const keySep = "\x1f"

// classKey is a call number's class as it sorts: none first, then numbers as numbers (153.42
// before 823.92), then a prefix as written.
func classKey(text string) string {
	if text == "" {
		return ""
	}
	if n, err := strconv.ParseFloat(text, 64); err == nil && n >= 0 && n < 1e15 {
		return "1" + numberKey(n)
	}
	return "2" + text
}

// volumeKey is a series number as it sorts: none first, then the volumes it holds as
// compareVolumes orders them, then words as written.
func volumeKey(n string) string {
	if n == "" {
		return "0"
	}
	if first, last, ok := volumes(n); ok {
		return "1" + numberKey(first) + numberKey(last) + n
	}
	return "2" + n
}

// numberKey writes a number to a fixed width, so numbers sort as text the way they sort as
// numbers.
func numberKey(n float64) string {
	return fmt.Sprintf("%026.10f", n)
}

// sortShelved puts things in the order their copies stand on the shelf.
func sortShelved[T any](items []T, shelving func(*T) *db.CopyShelving) {
	type placed struct {
		item T
		key  string
	}
	all := make([]placed, len(items))
	for i := range items {
		all[i] = placed{items[i], shelfKey(shelving(&items[i]))}
	}
	slices.SortStableFunc(all, func(a, b placed) int { return strings.Compare(a.key, b.key) })
	for i := range all {
		items[i] = all[i].item
	}
}
