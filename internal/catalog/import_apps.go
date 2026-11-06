package catalog

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"homelibrary/internal/lookup"
)

// Goodreads writes a series into the title: "Blood Song (Raven's Shadow, #1)".
var goodreadsSeriesRe = regexp.MustCompile(`^(.+?)\s*\(([^()]+?),?\s*#(\d+(?:\.\d+)?)\)$`)

// The shelves that mean a book is the owner's, as people name them on Goodreads.
var ownedShelves = map[string]bool{"own": true, "owned": true, "owned-books": true, "i-own": true, "books-i-own": true, "my-library": true, "my-books": true}

// goodreadsBook reads a row of Goodreads' export (My Books, then Import and export). A book is
// the owner's when Goodreads counts an owned copy or it sits on an "owned" shelf; a to-read
// book is wanted, not had. The other shelves the owner made become genres.
func goodreadsBook(get func(string) string) *importedBook {
	ref := get("Book Id")
	in := BookInput{
		Title:      get("Title"),
		ReadStatus: "unread",
		FinishedOn: day(get("Date Read")),
		Rating:     min(atoi(get("My Rating")), 5),
		Notes:      joinNotes(get("My Review"), get("Private Notes")),
		ISBN:       goodreadsISBN(get("ISBN13")),
		Publisher:  get("Publisher"),
		Published:  lookup.NormalizeDate(get("Year Published")),
		Format:     lookup.FormatFromText(get("Binding")),
		Pages:      atoi(get("Number of Pages")),
		Sources:    map[string]string{},
	}
	if in.ISBN == "" {
		in.ISBN = goodreadsISBN(get("ISBN"))
	}
	if m := goodreadsSeriesRe.FindStringSubmatch(in.Title); m != nil {
		in.Title, in.Series, in.SeriesNumber = m[1], strings.TrimSpace(m[2]), m[3]
	}
	if a := get("Author"); a != "" {
		in.Contributors = append(in.Contributors, lookup.Contributor{Name: a, Role: "author"})
	}
	for _, name := range splitNames(get("Additional Authors")) {
		in.Contributors = append(in.Contributors, lookup.Contributor{Name: name, Role: "contributor"})
	}
	shelf := get("Exclusive Shelf")
	switch shelf {
	case "read":
		in.ReadStatus = "read"
	case "currently-reading":
		in.ReadStatus = "reading"
	}
	owned := atoi(get("Owned Copies")) > 0
	for _, s := range splitList(get("Bookshelves")) {
		switch s = strings.ToLower(s); {
		case ownedShelves[s]:
			owned = true
		case s != "read" && s != "currently-reading" && s != "to-read" && s != shelf:
			in.Genres = append(in.Genres, s)
		}
	}
	imported(&in, ImportSourceGoodreads)
	b := &importedBook{
		Ref:    ref,
		In:     in,
		First:  CopyInput{AcquiredAs: "unknown", PriceCents: -1, Condition: "unknown", DustJacket: "unknown"},
		Copies: max(1, int(atoi(get("Owned Copies")))),
		Owned:  owned,
		Wanted: shelf == "to-read",
	}
	if ref != "" {
		b.Link = "https://www.goodreads.com/book/show/" + ref
	}
	return b
}

// goodreadsISBN undoes the spreadsheet formula Goodreads wraps an ISBN in: ="9780425268285".
func goodreadsISBN(s string) string {
	return strings.Trim(strings.TrimPrefix(strings.TrimSpace(s), "="), `"`)
}

var (
	isbnRunRe = regexp.MustCompile(`\b(97[89]\d{10}|\d{9}[\dXx])\b`)
)

// librarythingBook reads a row of LibraryThing's export (Tools, then Export), CSV or
// tab-delimited. A book is the owner's when it is in the "Your library" collection, or the
// export has no collections at all; one only on the wish list or to read is wanted.
func librarythingBook(get func(string) string) *importedBook {
	ref := get("Book Id")
	in := BookInput{
		Title:       get("Title"),
		ReadStatus:  "unread",
		StartedOn:   day(get("Date Started")),
		FinishedOn:  day(get("Date Read")),
		Rating:      ltRating(get("Rating")),
		Notes:       joinNotes(get("Review"), get("Comment"), get("Comments"), get("Private Comment")),
		Genres:      splitList(get("Tags")),
		ISBN:        firstISBN(get("ISBNs") + " " + get("ISBN")),
		LCCN:        get("LCCN"),
		Format:      lookup.FormatFromText(get("Media")),
		Pages:       atoi(get("Page Count")),
		Language:    lookup.NormalizeLanguage(firstOf(get("Languages"))),
		DDC:         get("Dewey Decimal"),
		LCC:         get("LC Classification"),
		Description: lookup.PlainText(get("Summary")),
		Sources:     map[string]string{},
	}
	// "Ace (2014), Edition: Reprint, 608 pages": the publisher, before the year.
	if pub := get("Publication"); pub != "" {
		if before, _, ok := strings.Cut(pub, " ("); ok {
			in.Publisher = strings.TrimSpace(before)
		}
	}
	if y := lookup.Year(get("Date")); y != "" {
		in.Published = y
	}
	in.HeightMM = int64(firstLength(get("Height")))
	in.Contributors = append(in.Contributors, ltPeople(get("Primary Author"), get("Primary Author Role"), "author")...)
	in.Contributors = append(in.Contributors, ltPeople(get("Secondary Author"), get("Secondary Author Roles"), "contributor")...)
	collections := map[string]bool{}
	for _, c := range splitList(get("Collections")) {
		collections[strings.ToLower(c)] = true
	}
	switch {
	case in.FinishedOn != "":
		in.ReadStatus = "read"
	case in.StartedOn != "" || collections["currently reading"]:
		in.ReadStatus = "reading"
	}
	imported(&in, ImportSourceLibraryThing)
	b := &importedBook{
		Ref: ref,
		In:  in,
		First: CopyInput{
			AcquiredAs:   "unknown",
			AcquiredOn:   day(get("Acquired")),
			AcquiredFrom: firstOf(get("From Where") + "," + get("Source")),
			PriceCents:   cents(get("Purchase Price")),
			Condition:    copyCondition(get("Condition")),
			DustJacket:   "unknown",
		},
		Copies: max(1, int(atoi(get("Copies")))),
		Owned:  len(collections) == 0 || collections["your library"],
		Wanted: collections["wishlist"] || collections["to read"],
	}
	if work := get("Work id"); ref != "" && work != "" {
		b.Link = "https://www.librarything.com/work/" + work + "/details/" + ref
	}
	return b
}

// firstISBN is the first ISBN among those written, the 13-digit one where both are.
func firstISBN(s string) string {
	var ten string
	for _, m := range isbnRunRe.FindAllString(s, -1) {
		if isbn, ok := lookup.NormalizeISBN(m); ok {
			if len(m) == 13 {
				return isbn
			}
			if ten == "" {
				ten = isbn
			}
		}
	}
	return ten
}

// ltRating is LibraryThing's rating, which may be in half stars, to the whole star below.
func ltRating(s string) int64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return min(int64(math.Floor(f)), 5)
}

// ltPeople reads LibraryThing's names, written last name first ("Ryan, Anthony") and several
// to a column with a bar between, with their roles beside them.
func ltPeople(names, roles, fallback string) []lookup.Contributor {
	var out []lookup.Contributor
	rs := strings.Split(roles, "|")
	for i, n := range strings.Split(names, "|") {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if last, first, ok := strings.Cut(n, ", "); ok && !strings.Contains(first, ",") {
			n = first + " " + last
		}
		role := fallback
		if i < len(rs) {
			role = contributorRole(rs[i], fallback)
		}
		out = append(out, lookup.Contributor{Name: n, Role: role})
	}
	return out
}

// contributorRole reads a role as an app names it into one the catalog keeps.
func contributorRole(s, fallback string) string {
	s = strings.ToLower(s)
	for _, r := range []string{"author", "editor", "translator", "illustrator", "narrator"} {
		if strings.Contains(s, r) {
			return r
		}
	}
	return fallback
}

func firstOf(list string) string {
	if l := splitList(list); len(l) > 0 {
		return l[0]
	}
	return ""
}

// firstLength reads a length with its unit ("9.25 inches", "24 cm") into millimetres.
func firstLength(s string) int {
	h, _, _ := lookup.Dimensions(s)
	return h
}
