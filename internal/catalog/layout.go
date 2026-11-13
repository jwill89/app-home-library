package catalog

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"homelibrary/internal/db"
)

// Shelf is one shelf of a bookcase, from the top down, with its inside measurements and the
// area it is in.
type Shelf struct {
	ID         int64  `json:"id"`
	BookcaseID int64  `json:"bookcaseId"`
	Bookcase   string `json:"bookcase"`
	Position   int64  `json:"position" doc:"From the top, 1 up"`
	AreaID     int64  `json:"areaId" doc:"The area it is in; 0 for none"`
	Area       string `json:"area" doc:"The area's name; empty for none"`
	ShelfInput
}

// ShelfInput is a shelf as the owner measures it.
type ShelfInput struct {
	WidthMM  int64 `json:"widthMm" minimum:"100" maximum:"5000" doc:"Inside the sides"`
	HeightMM int64 `json:"heightMm" minimum:"50" maximum:"1000" doc:"Clear space above the shelf"`
	DepthMM  int64 `json:"depthMm" minimum:"50" maximum:"1000"`
	// Corner braces under the shelf above, at both ends and the whole depth: a book standing
	// under one has the brace's height less room.
	BraceHeightMM int64 `json:"braceHeightMm" minimum:"0" maximum:"200" doc:"How far a corner brace hangs below the shelf above; 0 for none"`
	BraceWidthMM  int64 `json:"braceWidthMm" minimum:"0" maximum:"200" doc:"How far a corner brace reaches in from the side; 0 for none"`
}

// ShelfBook is a copy as the planner places it.
type ShelfBook struct {
	CopyID         int64    `json:"copyId"`
	BookID         int64    `json:"bookId"`
	Barcode        string   `json:"barcode"`
	Title          string   `json:"title"`
	CallNumber     []string `json:"callNumber"`
	Dewey          string   `json:"dewey" doc:"Its Dewey number: its own, or estimated from its LC class"`
	DeweyEstimated bool     `json:"deweyEstimated" doc:"The Dewey number is estimated from the LC class"`
	HeightMM       int64    `json:"heightMm"`
	ThicknessMM    int64    `json:"thicknessMm"`
	Estimated      bool     `json:"estimated" doc:"Its height or thickness is worked out from its format and pages"`
	OnLoan         bool     `json:"onLoan" doc:"Out on loan; its place on the shelf is kept for it"`
	facts          bookFacts
	key            areaKey
}

// ShelfPlan is a shelf with the books a layout puts on it.
type ShelfPlan struct {
	Shelf
	Books  []ShelfBook `json:"books"`
	UsedMM int64       `json:"usedMm" doc:"How far along the shelf the books reach, a gap at the start included"`
	GapMM  int64       `json:"gapMm" doc:"Space left empty at the start, where a corner brace is too low for the first book"`
	OverMM int64       `json:"overMm" doc:"How far the books run past what the shelf holds; 0 when they fit"`
	First  string      `json:"first" doc:"The first book by its area's first key: its Dewey number, author, series and volume, title, or year; the title where that is blank"`
	Last   string      `json:"last" doc:"The last book, as the first"`
	InUse  bool        `json:"inUse" doc:"Has a place in the layout in use; a shelf added or moved to another area since has none"`
}

// Layout is the library on its shelves: the layout in use, a suggestion asked for, or, with
// neither, the shelves with nothing on them.
type Layout struct {
	InUse       bool        `json:"inUse" doc:"The shelves keep the places a layout gave them"`
	Suggested   bool        `json:"suggested" doc:"A suggestion asked for, not yet put to use"`
	AppliedAt   string      `json:"appliedAt"`
	FillPercent int64       `json:"fillPercent"`
	Shelves     []ShelfPlan `json:"shelves"`
	NoArea      []ShelfBook `json:"noArea" doc:"Books no area's rules take"`
	Unplaced    []ShelfBook `json:"unplaced" doc:"Books their area has no room for, or no shelves"`
	TooTall     []ShelfBook `json:"tooTall" doc:"Books taller than the shelf their turn came on in their area"`
	Estimated   int64       `json:"estimated" doc:"Books whose size is an estimate"`
}

// bookFacts are what an area's rules and order ask of a book.
type bookFacts struct {
	category   string
	dewey      float64
	hasDewey   bool
	subjects   []string
	authorIDs  []int64
	seriesID   int64
	author     string // the first author's sort name, as filed
	authorName string // the same, as written, for a shelf's edge
	series     string
	seriesName string
	volume     string
	title      string
	published  string
}

// bookSize is a copy's height and thickness on the shelf: measured where the sources gave it,
// worked out otherwise from its format and pages, as the owner's own measured books bear out
// (about 0.065 mm a page for a paperback, less for a hardcover's paper but with its boards).
func bookSize(r *db.ListShelfCopiesRow) (height, thickness int64, estimated bool) {
	height, thickness = r.HeightMm, r.ThicknessMm
	if height == 0 {
		estimated = true
		height = map[string]int64{"mass_market": 175, "paperback": 212, "trade_paperback": 223, "hardcover": 246, "library_binding": 246, "leather": 240}[r.Format]
		if height == 0 {
			height = 220
		}
	}
	if thickness == 0 {
		estimated = true
		perPage, boards := 0.065, 3.0
		switch r.Format {
		case "hardcover", "library_binding", "leather":
			perPage, boards = 0.05, 6
		case "mass_market":
			perPage, boards = 0.05, 2
		}
		pages := float64(r.Pages)
		if pages == 0 {
			pages = 300
		}
		thickness = int64(pages*perPage + boards + 0.5)
	}
	return height, thickness, estimated
}

// shelfBooks is every copy in the house as the planner sees it, by title until an area orders
// it. authors are each edition's authors.
func shelfBooks(rows []*db.ListShelfCopiesRow, authors map[int64][]int64) []ShelfBook {
	out := make([]ShelfBook, 0, len(rows))
	for _, r := range rows {
		shelving := &db.CopyShelving{CopyID: r.CopyID, EditionID: r.EditionID, Section: r.Section, Shelving: r.Shelving,
			LabelPrefix: r.LabelPrefix, Ddc: r.Ddc, Title: r.Title, Series: r.Series, SeriesNumber: r.SeriesNumber, AuthorSort: r.AuthorSort}
		var subjects []string
		_ = json.Unmarshal([]byte(r.Subjects), &subjects) // written by this package as a JSON list
		category, _ := categoryOf(r.Category, r.Ddc, r.Series, r.Title, r.Publisher, subjects)
		f := bookFacts{
			category: category, subjects: lowered(subjects), authorIDs: authors[r.EditionID], seriesID: r.SeriesID,
			author: strings.ToLower(r.AuthorSort), authorName: r.AuthorSort, series: strings.ToLower(withoutArticle(r.Series)),
			seriesName: r.Series, volume: r.SeriesNumber,
			title: strings.ToLower(withoutArticle(r.Title)), published: r.Published,
		}
		dewey, estimated := r.Ddc, false
		if dewey == "" {
			dewey, estimated = estimateDewey(r.Lcc)
			estimated = estimated && dewey != ""
		}
		if dewey != "" {
			f.dewey, f.hasDewey = deweyValue(dewey), true
		}
		h, t, est := bookSize(r)
		out = append(out, ShelfBook{
			CopyID: r.CopyID, BookID: r.EditionID, Barcode: Barcode(r.CopyID), Title: r.Title, CallNumber: CallNumber(shelving),
			Dewey: dewey, DeweyEstimated: estimated, HeightMM: h, ThicknessMM: t, Estimated: est, OnLoan: r.OnLoan == 1, facts: f,
		})
	}
	return out
}

func lowered(list []string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = strings.ToLower(s)
	}
	return out
}

// takes says whether an area's rules take a book: every rule set holds, and any one entry of
// a list. A range ending on a whole number takes that number's decimals: 500 to 599 takes 599.9.
func (a *Area) takes(b *ShelfBook) bool {
	f := &b.facts
	if len(a.Categories) > 0 && !slices.Contains(a.Categories, f.category) {
		return false
	}
	if a.DeweyFrom != "" || a.DeweyTo != "" {
		if !f.hasDewey || (a.DeweyFrom != "" && f.dewey < deweyValue(a.DeweyFrom)) {
			return false
		}
		if a.DeweyTo != "" {
			to := deweyValue(a.DeweyTo)
			if !strings.Contains(a.DeweyTo, ".") {
				to++
				if f.dewey >= to {
					return false
				}
			} else if f.dewey > to {
				return false
			}
		}
	}
	if a.Subject != "" {
		words := strings.ToLower(a.Subject)
		if !slices.ContainsFunc(f.subjects, func(s string) bool { return strings.Contains(s, words) }) {
			return false
		}
	}
	if len(a.authorIDs) > 0 && !slices.ContainsFunc(f.authorIDs, func(id int64) bool { return slices.Contains(a.authorIDs, id) }) {
		return false
	}
	if len(a.seriesIDs) > 0 && !slices.Contains(a.seriesIDs, f.seriesID) {
		return false
	}
	if a.MinHeightMM > 0 && b.HeightMM < a.MinHeightMM {
		return false
	}
	return a.MaxHeightMM == 0 || b.HeightMM <= a.MaxHeightMM
}

// keyPart is one step of a book's place in an area's order. A book without the detail comes
// after those with it; numbers compare as numbers, and before words.
type keyPart struct {
	Missing bool    `json:"m,omitempty"`
	IsNum   bool    `json:"n,omitempty"`
	Num     float64 `json:"v,omitempty"`
	Text    string  `json:"t,omitempty"`
}

type areaKey []keyPart

func numPart(v float64) keyPart { return keyPart{IsNum: true, Num: v} }

func textPart(s string) keyPart {
	if s == "" {
		return keyPart{Missing: true}
	}
	return keyPart{Text: s}
}

// volumePart is a series number as it sorts: 2 before 10, and a number before a word.
func volumePart(s string) keyPart {
	if n, err := strconv.ParseFloat(strings.TrimPrefix(s, "v."), 64); err == nil {
		return numPart(n)
	}
	return textPart(s)
}

func (a keyPart) compare(b keyPart) int {
	switch {
	case a.Missing || b.Missing:
		return cmp.Compare(boolInt(a.Missing), boolInt(b.Missing))
	case a.IsNum && b.IsNum:
		return cmp.Compare(a.Num, b.Num)
	case a.IsNum != b.IsNum:
		return cmp.Compare(boolInt(b.IsNum), boolInt(a.IsNum))
	}
	return cmp.Compare(a.Text, b.Text)
}

func (k areaKey) compare(o areaKey) int {
	for i := range min(len(k), len(o)) {
		if c := k[i].compare(o[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(k), len(o))
}

// keyFor is a book's place in an area's order: its keys in turn, then its title and copy, so
// no two books tie.
func keyFor(b *ShelfBook, order []string) areaKey {
	f := &b.facts
	key := make(areaKey, 0, len(order)+3)
	for _, k := range order {
		switch k {
		case "subject":
			if f.hasDewey {
				key = append(key, numPart(f.dewey))
			} else {
				key = append(key, keyPart{Missing: true})
			}
		case "author":
			key = append(key, textPart(f.author))
		case "series":
			if f.series == "" {
				key = append(key, keyPart{Missing: true}, keyPart{Missing: true})
			} else {
				key = append(key, textPart(f.series), volumePart(f.volume))
			}
		case "title":
			key = append(key, textPart(f.title))
		case "published":
			key = append(key, textPart(f.published))
		case "height":
			key = append(key, numPart(float64(b.HeightMM)))
		}
	}
	return append(key, textPart(f.title), numPart(float64(b.CopyID)))
}

// rangeText is how a shelf's first or last book is named on its edge: by the area's first key,
// or its title where that is blank or unsuited to a label.
func rangeText(b *ShelfBook, order []string) string {
	f := &b.facts
	first := ""
	if len(order) > 0 {
		first = order[0]
	}
	switch first {
	case "subject":
		if b.Dewey != "" && b.DeweyEstimated {
			return "~" + b.Dewey
		}
		if b.Dewey != "" {
			return b.Dewey
		}
	case "author":
		if surname, _, _ := strings.Cut(f.authorName, ","); surname != "" {
			return surname
		}
	case "series":
		if f.seriesName != "" && f.volume != "" {
			return f.seriesName + " v." + f.volume
		}
		if f.seriesName != "" {
			return f.seriesName
		}
	case "published":
		if len(f.published) >= 4 {
			return f.published[:4]
		}
	}
	return b.Title
}

func (p *ShelfPlan) braced() bool {
	return p.BraceHeightMM > 0 && p.BraceWidthMM > 0
}

// underBrace reports whether a book is short enough to stand under a corner brace.
func (p *ShelfPlan) underBrace(b ShelfBook) bool {
	return !p.braced() || b.HeightMM <= p.HeightMM-p.BraceHeightMM
}

// startFor is where a book would stand: after the books already there, or, for one too tall
// for the brace at the start of the shelf, just past the brace.
func (p *ShelfPlan) startFor(b ShelfBook) int64 {
	if p.UsedMM < p.BraceWidthMM && !p.underBrace(b) {
		return p.BraceWidthMM
	}
	return p.UsedMM
}

// endFor is how far along a book may reach: the far side, or the brace there for a tall one.
func (p *ShelfPlan) endFor(b ShelfBook) int64 {
	if p.underBrace(b) {
		return p.WidthMM
	}
	return p.WidthMM - p.BraceWidthMM
}

func (p *ShelfPlan) put(b ShelfBook) {
	start := p.startFor(b)
	p.GapMM += start - p.UsedMM
	p.UsedMM = start + b.ThicknessMM
	p.Books = append(p.Books, b)
}

func (p *ShelfPlan) room(b ShelfBook, fill int64) bool {
	end := p.startFor(b) + b.ThicknessMM
	if end > p.endFor(b) {
		return false
	}
	// An empty shelf takes any book that stands on it, whatever the fill.
	return end <= p.WidthMM*fill/100 || len(p.Books) == 0
}

// sorted is the books each area takes, in its order, and those no area takes. A book goes to
// the first area, in the owner's order, whose rules take it.
func sorted(areas []Area, books []ShelfBook) (byArea map[int64][]ShelfBook, noArea []ShelfBook) {
	byArea = map[int64][]ShelfBook{}
	for _, b := range books {
		i := slices.IndexFunc(areas, func(a Area) bool { return a.takes(&b) })
		if i < 0 {
			noArea = append(noArea, b)
			continue
		}
		b.key = keyFor(&b, areas[i].Order)
		byArea[areas[i].ID] = append(byArea[areas[i].ID], b)
	}
	for id := range byArea {
		slices.SortStableFunc(byArea[id], func(a, b ShelfBook) int { return a.key.compare(b.key) })
	}
	slices.SortStableFunc(noArea, func(a, b ShelfBook) int { return cmp.Compare(a.facts.title, b.facts.title) })
	return byArea, noArea
}

// shelvesOf is the places in plans of an area's shelves, in walking order.
func shelvesOf(plans []ShelfPlan, area int64) []int {
	var out []int
	for i := range plans {
		if plans[i].AreaID == area {
			out = append(out, i)
		}
	}
	return out
}

// propose lays each area's books, in its order, along its shelves in walking order, each shelf
// filled to fill percent of its width. A book taller than the shelf its turn comes on is set
// aside as too tall, and the next book still takes that shelf; an area for tall books, tried
// first, is where such books belong.
func propose(plans []ShelfPlan, areas []Area, byArea map[int64][]ShelfBook, fill int64) (unplaced, tooTall []ShelfBook) {
	for _, a := range areas {
		shelves := shelvesOf(plans, a.ID)
		i := 0
		for _, b := range byArea[a.ID] {
			for {
				if i >= len(shelves) {
					unplaced = append(unplaced, b)
					break
				}
				p := &plans[shelves[i]]
				if b.HeightMM > p.HeightMM {
					tooTall = append(tooTall, b)
					break
				}
				if p.room(b, fill) {
					p.put(b)
					break
				}
				i++
			}
		}
	}
	return unplaced, tooTall
}

// shelfStart is where a shelf begins in the layout in use: its area, the area's order then,
// and the first book's place in it. A shelf whose area or order has changed since has none.
type shelfStart struct {
	Area  int64   `json:"area"`
	Order string  `json:"order"`
	Key   areaKey `json:"key"`
}

func startOf(area *Area, b ShelfBook) string {
	j, _ := json.Marshal(shelfStart{Area: area.ID, Order: strings.Join(area.Order, ","), Key: b.key})
	return string(j)
}

// startFrom is a shelf's start as saved, if it still holds for the area the shelf is in.
func startFrom(saved string, area *Area) (areaKey, bool) {
	var s shelfStart
	if area == nil || strings.TrimSpace(saved) == "" || json.Unmarshal([]byte(saved), &s) != nil {
		return nil, false
	}
	if s.Area != area.ID || s.Order != strings.Join(area.Order, ",") {
		return nil, false
	}
	return s.Key, true
}

// hold puts each book on the shelf of its area whose place in the area's order it falls in:
// the last that starts at or before it, or the first. A shelf takes what belongs there even past
// its width, which the map then shows as over full. A book taller than its shelf is set aside.
func hold(plans []ShelfPlan, starts map[int64]areaKey, areas []Area, byArea map[int64][]ShelfBook) (unplaced, tooTall []ShelfBook) {
	for _, a := range areas {
		var held []int
		for _, i := range shelvesOf(plans, a.ID) {
			if _, ok := starts[plans[i].ID]; ok {
				held = append(held, i)
			}
		}
		for _, b := range byArea[a.ID] {
			if len(held) == 0 {
				unplaced = append(unplaced, b)
				continue
			}
			at := held[0]
			for _, i := range held {
				if starts[plans[i].ID].compare(b.key) <= 0 {
					at = i
				}
			}
			if b.HeightMM > plans[at].HeightMM {
				tooTall = append(tooTall, b)
				continue
			}
			plans[at].put(b)
		}
	}
	return unplaced, tooTall
}

// finish names each shelf's first and last books, by its area's first key, and how far its
// books run past what it holds: the far side, or the brace there when the last is too tall.
func finish(plans []ShelfPlan, areas []Area) {
	orders := map[int64][]string{}
	for _, a := range areas {
		orders[a.ID] = a.Order
	}
	for i := range plans {
		p := &plans[i]
		if p.Books == nil {
			p.Books = []ShelfBook{}
		}
		if n := len(p.Books); n > 0 {
			p.First, p.Last = rangeText(&p.Books[0], orders[p.AreaID]), rangeText(&p.Books[n-1], orders[p.AreaID])
			p.OverMM = max(0, p.UsedMM-p.endFor(p.Books[n-1]))
		}
	}
}
