package catalog

import (
	"context"
	"slices"
)

// Scope is what a share link shows of the library: its books (every one, those of the
// categories chosen and the books chosen one by one, or none), with the wanted ones among
// them or not, and its papers (every one, a course's and the papers chosen, or none). The
// owner's own requests have no scope, a nil one, and see everything.
type Scope struct {
	Books      string   `json:"books" enum:"all,some,none" doc:"Every book, those of the categories and the books chosen, or none"`
	Categories []string `json:"categories" maxItems:"6" enum:"fiction,nonfiction,manga,light_novel,graphic_novel,uncategorized"`
	BookIDs    []int64  `json:"bookIds" maxItems:"5000" doc:"Books shown whatever their category"`
	Wishlist   bool     `json:"wishlist" doc:"The wanted books among those shown"`
	Papers     string   `json:"papers" enum:"all,some,none" doc:"Every paper, those of the courses and the papers chosen, or none"`
	CourseIDs  []int64  `json:"courseIds" maxItems:"500" doc:"Courses whose papers are shown; never named to a visitor"`
	PaperIDs   []int64  `json:"paperIds" maxItems:"5000" doc:"Papers shown whatever their course"`
}

// Sees is what a link's pages have to show, so a visitor is offered only those.
type Sees struct {
	Books    bool `json:"books" doc:"The books, their series, and their reading"`
	Wishlist bool `json:"wishlist"`
	Papers   bool `json:"papers"`
}

// Sees says which of the library's pages the link has something for.
func (sc *Scope) Sees() Sees {
	if sc == nil {
		return Sees{Books: true, Wishlist: true, Papers: true}
	}
	books := sc.Books != "none"
	return Sees{Books: books, Wishlist: books && sc.Wishlist, Papers: sc.Papers != "none"}
}

// Normalize makes a scope as the owner chose it ready to keep: choices only where some are
// chosen, each once, and a link that shows something.
func (sc *Scope) Normalize() error {
	if sc.Books != "some" {
		sc.Categories, sc.BookIDs = []string{}, []int64{}
	}
	if sc.Books == "none" {
		sc.Wishlist = false
	}
	if sc.Papers != "some" {
		sc.CourseIDs, sc.PaperIDs = []int64{}, []int64{}
	}
	sc.Categories = slices.Compact(slices.Sorted(slices.Values(sc.Categories)))
	for _, ids := range []*[]int64{&sc.BookIDs, &sc.CourseIDs, &sc.PaperIDs} {
		*ids = slices.Compact(slices.Sorted(slices.Values(*ids)))
	}
	switch {
	case sc.Books == "some" && len(sc.Categories) == 0 && len(sc.BookIDs) == 0:
		return &FieldError{"scope.books", "Choose a category or a book to share, or share every book or none."}
	case sc.Papers == "some" && len(sc.CourseIDs) == 0 && len(sc.PaperIDs) == 0:
		return &FieldError{"scope.papers", "Choose a course or a paper to share, or share every paper or none."}
	case sc.Books == "none" && sc.Papers == "none":
		return &FieldError{"scope", "A link has to share something: some books, or some papers."}
	}
	return nil
}

// showsBook is the one rule for the books a link shows. A wanted book shows with the wishlist;
// any other while a copy of it is in the house, since one whose copies have all gone is the
// owner's record alone. Then the link's books: every one, or those of its categories and its
// own choices.
func (sc *Scope) showsBook(id int64, category string, wanted, here bool) bool {
	if sc == nil {
		return true
	}
	if (wanted && !sc.Wishlist) || (!wanted && !here) {
		return false
	}
	switch sc.Books {
	case "all":
		return true
	case "some":
		return slices.Contains(sc.Categories, category) || slices.Contains(sc.BookIDs, id)
	}
	return false
}

// ShowsBook reports whether the link shows a book.
func (sc *Scope) ShowsBook(b *Book) bool {
	here := slices.ContainsFunc(b.Copies, func(c Copy) bool { return c.GoneAs == "" })
	return sc.showsBook(b.ID, b.Category, b.WantedOn != "", here)
}

// Rows is the library's list as the link shows it.
func (sc *Scope) Rows(rows []BookRow) []BookRow {
	if sc == nil {
		return rows
	}
	out := make([]BookRow, 0, len(rows))
	shown := map[int64]bool{}
	for _, r := range rows {
		if sc.showsBook(r.ID, r.Category, r.WantedOn != "", r.Copies > 0) {
			out = append(out, r)
			shown[r.ID] = true
		}
	}
	// A wanted book had in another edition names that edition only where the link shows it.
	for i := range out {
		if !shown[out[i].HadAs] {
			out[i].HadAs = 0
		}
	}
	return out
}

// shown is the set of books a link shows, for the pages that name books by number (the
// series, the reading); nil for the owner, who sees every one.
func (s *Service) shown(ctx context.Context, sc *Scope) (map[int64]bool, error) {
	if sc == nil {
		return nil, nil
	}
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	set := map[int64]bool{}
	for _, r := range sc.Rows(rows) {
		set[r.ID] = true
	}
	return set, nil
}

// ShowsPaper reports whether the link shows a paper: every one, or those of its courses and
// its own choices.
func (sc *Scope) ShowsPaper(id, courseID int64) bool {
	if sc == nil {
		return true
	}
	switch sc.Papers {
	case "all":
		return true
	case "some":
		return slices.Contains(sc.PaperIDs, id) || (courseID > 0 && slices.Contains(sc.CourseIDs, courseID))
	}
	return false
}
