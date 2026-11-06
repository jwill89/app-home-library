package catalog

import (
	"strings"

	"homelibrary/internal/lookup"
)

// workKey is what two editions of one work share however each writes it: the title up to any
// subtitle or note (":", ";", ",", "(", "[", "."), without case, punctuation, or a leading
// article, and the first author's surname. Empty when either is missing: a title alone
// ("Selections", "Epistemology") is too often two works.
func workKey(title, firstAuthor string) string {
	if i := strings.IndexAny(title, ":;,([."); i > 0 {
		title = title[:i]
	}
	words := keyWords(title)
	if len(words) > 1 && isArticle(words[0]) {
		words = words[1:]
	}
	surname := keyWords(lookup.Surname(firstAuthor))
	if len(words) == 0 || len(surname) == 0 {
		return ""
	}
	return strings.Join(words, " ") + "|" + strings.Join(surname, " ")
}

// firstAuthor is a book's first author, as its contributors name them.
func firstAuthor(people []lookup.Contributor) string {
	for _, c := range people {
		if c.Role == "author" {
			return c.Name
		}
	}
	return ""
}

// listedFirst is the first of the authors a list row names, as "A, B".
func listedFirst(authors string) string {
	first, _, _ := strings.Cut(authors, ", ")
	return first
}

// ownedWorks is every work the house has a copy of, by its key, with one of its editions.
func ownedWorks(rows []BookRow) map[string]int64 {
	out := map[string]int64{}
	for _, r := range rows {
		if r.WantedOn != "" {
			continue
		}
		if k := workKey(r.Title, listedFirst(r.Authors)); k != "" && out[k] == 0 {
			out[k] = r.ID
		}
	}
	return out
}
