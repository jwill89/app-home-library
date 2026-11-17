package catalog

import (
	"slices"
	"strings"
	"unicode"
)

// Categories a book may be in, as the Library page offers them.
var Categories = []string{"fiction", "nonfiction", "textbook", "manga", "light_novel", "graphic_novel", "uncategorized"}

// sourceInferred marks a category worked out from a book's details rather than set, and
// sourceCourse a textbook's, worked out from a copy of it being on a course: either is kept
// empty, so it is worked out again as the details, the courses, or the rule change.
const (
	sourceInferred = "inferred"
	sourceCourse   = "course"
)

// workedOut reports whether a category came from the app rather than the owner or an import.
func workedOut(source string) bool { return source == sourceInferred || source == sourceCourse }

// Publishers whose books say what they are, where nothing else does. A bare name that two
// houses share is left out: Image Books publishes Copleston's history of philosophy, Image
// Comics Saga.
var (
	lightNovelImprints = []string{"j-novel club", "j-novel heart", "yen on", "cross infinite world", "airship"}
	mangaPublishers    = []string{"viz", "viz media", "kodansha", "kodansha comics", "seven seas", "yen press", "square enix", "tokyopop", "vertical comics", "denpa", "ghost ship", "one peace books", "udon entertainment", "スクウェア・エニックス"}
	comicsPublishers   = []string{"image comics", "dark horse", "dark horse books", "dark horse comics", "dc comics", "marvel", "idw publishing", "boom! studios", "oni press", "fantagraphics", "drawn & quarterly", "top shelf", "first second"}
	fictionImprints    = []string{"tor", "tor books", "tor fantasy", "del rey", "47north", "grim oak press", "orbit", "ace", "ace books", "daw", "gollancz", "harper voyager", "subterranean press", "bantam spectra", "baen", "angry robot", "titan books"}
	scholarlyPresses   = []string{"university press", "university of chicago press", "mit press", "routledge", "springer", "blackwell", "wiley", "free press", "basic books", "scientific american", "oxford university press", "cambridge university press"}
)

// inferCategory works a book's category out from its class number, series, title, publisher,
// and subjects (Google's categories among them, "Comics & Graphic Novels / Manga"); one it
// cannot tell is uncategorized. Light novels come first: they are classed and sold as fiction.
// The class number outweighs the subjects, and both outweigh who published it: Dark Horse
// publishes prose novels too, and Image Books Copleston.
func inferCategory(ddc, series, title, publisher string, subjects []string) string {
	named := strings.ToLower(series + " " + title)
	about := strings.ToLower(strings.Join(subjects, " | "))
	// Google files Western comics under "Graphic Novels & Manga": that names no manga.
	mangaAbout := strings.Contains(strings.ReplaceAll(about, "graphic novels & manga", ""), "manga")
	comicsAbout := strings.Contains(about, "graphic novel") || strings.Contains(about, "comic books, strips") ||
		strings.Contains(about, "comics & graphic novels")
	fictional := strings.Contains(about, "fiction") && !strings.Contains(about, "nonfiction") ||
		strings.Contains(about, "novel") || strings.Contains(about, "short stories")
	switch {
	case strings.Contains(named, "light novel") || strings.Contains(about, "light novel") || publishedBy(publisher, lightNovelImprints):
		return "light_novel"
	// The class number says whose comics: 741.5952 Japan's, 741.5973 America's, and so on.
	case strings.HasPrefix(ddc, "741.5952"):
		return "manga"
	case strings.HasPrefix(ddc, "741.59"):
		return "graphic_novel"
	case strings.Contains(strings.ToLower(series), "manga") || mangaAbout || strings.Contains(title, "コミック"):
		return "manga"
	case strings.HasPrefix(ddc, "741.5") || comicsAbout:
		return "graphic_novel"
	case ddc != "":
		// Literature (810–899) but its general works (800–809) is fiction, poetry, and drama.
		if len(ddc) >= 2 && ddc[0] == '8' && ddc[1] != '0' {
			return "fiction"
		}
		return "nonfiction"
	case publishedBy(publisher, mangaPublishers):
		return "manga"
	case publishedBy(publisher, comicsPublishers):
		return "graphic_novel"
	case about != "":
		if fictional {
			return "fiction"
		}
		return "nonfiction"
	case publishedBy(publisher, fictionImprints):
		return "fiction"
	case publishedBy(publisher, scholarlyPresses) || strings.Contains(strings.ToLower(series), "very short introduction"):
		return "nonfiction"
	}
	return "uncategorized"
}

// words is a name as publishedBy compares it: lower case, in words, its punctuation dropped
// but what a publisher's name keeps ("Drawn & Quarterly", "BOOM! Studios", "J-Novel Club").
func words(s string) string {
	w := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '&' && r != '!' && r != '-'
	})
	return " " + strings.Join(w, " ") + " "
}

// publishedBy reports whether a publisher is one of names, word for word: "Tor Books" is Tor,
// "History Press" is not.
func publishedBy(publisher string, names []string) bool {
	padded := words(publisher)
	return slices.ContainsFunc(names, func(n string) bool { return strings.Contains(padded, words(n)) })
}

// categoryOf is a book's category as it is shown, and the source of one not set: the one set;
// a textbook where a copy of it is on a course, since the owner kept it from one; or the one
// worked out from its details.
func categoryOf(stored string, onCourse bool, ddc, series, title, publisher string, subjects []string) (category, source string) {
	switch {
	case stored != "":
		return stored, ""
	case onCourse:
		return "textbook", sourceCourse
	}
	return inferCategory(ddc, series, title, publisher, subjects), sourceInferred
}

// storedCategory is the category a save keeps: none, to be worked out, unless it was set.
func storedCategory(in *BookInput) string {
	if in.Category == "" || workedOut(in.Sources["category"]) {
		return ""
	}
	return in.Category
}

// shelfCategory reads a shelf of the Bookshelf app the way the owner sorted the library before
// this one (Fiction, Nonfiction, Manga and Light Novels); empty for a shelf of another name.
func shelfCategory(shelf string, in *BookInput) string {
	switch strings.ToLower(strings.TrimSpace(shelf)) {
	case "fiction":
		return "fiction"
	case "nonfiction", "non-fiction":
		return "nonfiction"
	case "manga and light novels", "manga & light novels", "manga":
		if inferCategory("", in.Series, in.Title, in.Publisher, in.Subjects) == "light_novel" {
			return "light_novel"
		}
		return "manga"
	case "light novels":
		return "light_novel"
	case "graphic novels", "comics":
		return "graphic_novel"
	case "textbooks", "textbook":
		return "textbook"
	}
	return ""
}
