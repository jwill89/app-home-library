package catalog

import (
	"encoding/json"
	"strings"

	"homelibrary/internal/db"
)

// Shelf is one shelf of a bookcase, from the top down, with its inside measurements.
type Shelf struct {
	ID         int64  `json:"id"`
	BookcaseID int64  `json:"bookcaseId"`
	Bookcase   string `json:"bookcase"`
	Position   int64  `json:"position" doc:"From the top, 1 up"`
	ShelfInput
}

// ShelfInput is a shelf as the owner measures it.
type ShelfInput struct {
	WidthMM  int64 `json:"widthMm" minimum:"100" maximum:"5000" doc:"Inside the sides"`
	HeightMM int64 `json:"heightMm" minimum:"50" maximum:"1000" doc:"Clear space above the shelf"`
	DepthMM  int64 `json:"depthMm" minimum:"50" maximum:"1000"`
	Oversize bool  `json:"oversize" doc:"Takes the books too tall for the shelf their call number would put them on"`
	// Corner braces under the shelf above, at both ends and the whole depth: a book standing
	// under one has the brace's height less room.
	BraceHeightMM int64 `json:"braceHeightMm" minimum:"0" maximum:"200" doc:"How far a corner brace hangs below the shelf above; 0 for none"`
	BraceWidthMM  int64 `json:"braceWidthMm" minimum:"0" maximum:"200" doc:"How far a corner brace reaches in from the side; 0 for none"`
}

// ShelfBook is a copy as the planner places it.
type ShelfBook struct {
	CopyID      int64    `json:"copyId"`
	BookID      int64    `json:"bookId"`
	Barcode     string   `json:"barcode"`
	Title       string   `json:"title"`
	CallNumber  []string `json:"callNumber"`
	HeightMM    int64    `json:"heightMm"`
	ThicknessMM int64    `json:"thicknessMm"`
	Estimated   bool     `json:"estimated" doc:"Its height or thickness is worked out from its format and pages"`
	OnLoan      bool     `json:"onLoan" doc:"Out on loan; its place on the shelf is kept for it"`
	key         *db.CopyShelving
}

// ShelfPlan is a shelf with the books a layout puts on it.
type ShelfPlan struct {
	Shelf
	Books  []ShelfBook `json:"books"`
	UsedMM int64       `json:"usedMm" doc:"How far along the shelf the books reach, a gap at the start included"`
	GapMM  int64       `json:"gapMm" doc:"Space left empty at the start, where a corner brace is too low for the first book"`
	OverMM int64       `json:"overMm" doc:"How far the books run past what the shelf holds; 0 when they fit"`
	First  []string    `json:"first" doc:"The first book's call number"`
	Last   []string    `json:"last" doc:"The last book's call number"`
	InUse  bool        `json:"inUse" doc:"Has a place in the layout in use; a shelf added since has none"`
}

// Layout is the library on its shelves: the layout in use, or a proposal.
type Layout struct {
	InUse       bool        `json:"inUse" doc:"The shelves keep the places a layout gave them; false is a proposal"`
	AppliedAt   string      `json:"appliedAt"`
	FillPercent int64       `json:"fillPercent"`
	Shelves     []ShelfPlan `json:"shelves"`
	Unplaced    []ShelfBook `json:"unplaced" doc:"Books no shelf has room for"`
	TooTall     []ShelfBook `json:"tooTall" doc:"Books taller than every shelf that could take them"`
	Estimated   int64       `json:"estimated" doc:"Books whose size is an estimate"`
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

func shelfBooks(rows []*db.ListShelfCopiesRow) []ShelfBook {
	out := make([]ShelfBook, 0, len(rows))
	for _, r := range rows {
		key := &db.CopyShelving{CopyID: r.CopyID, EditionID: r.EditionID, Section: r.Section, Shelving: r.Shelving,
			LabelPrefix: r.LabelPrefix, Ddc: r.Ddc, Title: r.Title, Series: r.Series, SeriesNumber: r.SeriesNumber, AuthorSort: r.AuthorSort}
		h, t, est := bookSize(r)
		out = append(out, ShelfBook{
			CopyID: r.CopyID, BookID: r.EditionID, Barcode: Barcode(r.CopyID), Title: r.Title, CallNumber: CallNumber(key),
			HeightMM: h, ThicknessMM: t, Estimated: est, OnLoan: r.OnLoan == 1, key: key,
		})
	}
	sortShelved(out, func(b *ShelfBook) *db.CopyShelving { return b.key })
	return out
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

// propose lays the books (in shelf order) along the shelves (in walking order), each shelf
// filled to fill percent of its width. A book taller than the shelf its turn comes on goes to
// the oversize shelves, in its own order there; the next book still takes that shelf.
func propose(plans []ShelfPlan, books []ShelfBook, fill int64) (unplaced, tooTall []ShelfBook) {
	var tall []ShelfBook
	i := 0
	regular := shelvesWhere(plans, false)
	for _, b := range books {
		for {
			if i >= len(regular) {
				// Past the last shelf, a book too tall for it still goes the oversize way.
				if len(regular) == 0 || b.HeightMM > plans[regular[len(regular)-1]].HeightMM {
					tall = append(tall, b)
				} else {
					unplaced = append(unplaced, b)
				}
				break
			}
			p := &plans[regular[i]]
			if b.HeightMM > p.HeightMM {
				tall = append(tall, b)
				break
			}
			if p.room(b, fill) {
				p.put(b)
				break
			}
			i++
		}
	}
	u, t := fillOversize(plans, tall, fill)
	return append(unplaced, u...), t
}

// fillOversize puts the tall books on the oversize shelves in order, each to the fill, a book
// too tall for one oversize shelf trying the next.
func fillOversize(plans []ShelfPlan, tall []ShelfBook, fill int64) (unplaced, tooTall []ShelfBook) {
	over := shelvesWhere(plans, true)
	j := 0
	for _, b := range tall {
		fits, placed := false, false
		for k := j; k < len(over) && !placed; k++ {
			p := &plans[over[k]]
			if b.HeightMM > p.HeightMM {
				continue
			}
			fits = true
			if p.room(b, fill) {
				p.put(b)
				placed = true
			} else if k == j {
				j++ // the current oversize shelf is full
			}
		}
		switch {
		case placed:
		case fits || anyTallEnough(plans, over, b):
			unplaced = append(unplaced, b)
		default:
			tooTall = append(tooTall, b)
		}
	}
	return unplaced, tooTall
}

func anyTallEnough(plans []ShelfPlan, over []int, b ShelfBook) bool {
	for _, k := range over {
		if b.HeightMM <= plans[k].HeightMM {
			return true
		}
	}
	return false
}

func shelvesWhere(plans []ShelfPlan, oversize bool) []int {
	var out []int
	for i, p := range plans {
		if p.Oversize == oversize {
			out = append(out, i)
		}
	}
	return out
}

// hold puts each book on the shelf whose place in the order it falls in: the last shelf that
// starts at or before it. A shelf takes what belongs there even past its width, which the map
// then shows as over full; the tall books go to the oversize shelves as in a proposal, by their
// own starts when those shelves have them.
func hold(plans []ShelfPlan, starts map[int64]*db.CopyShelving, books []ShelfBook, fill int64) (unplaced, tooTall []ShelfBook) {
	// Each shelf's start, in order to compare against every book.
	from := make(map[int64]shelfOrder, len(starts))
	for id, s := range starts {
		from[id] = orderOf(s)
	}
	var tall []ShelfBook
	place := func(b ShelfBook, oversize bool) bool {
		at := -1
		order := orderOf(b.key)
		for i := range plans {
			s, ok := from[plans[i].ID]
			if plans[i].Oversize != oversize || !ok {
				continue
			}
			if at == -1 || s.compare(order) <= 0 {
				at = i
			}
		}
		if at == -1 {
			return false
		}
		if b.HeightMM > plans[at].HeightMM {
			if oversize {
				return false
			}
			tall = append(tall, b)
			return true
		}
		plans[at].put(b)
		return true
	}
	for _, b := range books {
		if !place(b, false) {
			unplaced = append(unplaced, b)
		}
	}
	if len(shelvesWhereHeld(plans, starts, true)) == 0 {
		u, t := fillOversize(plans, tall, fill)
		return append(unplaced, u...), t
	}
	for _, b := range tall {
		if !place(b, true) {
			tooTall = append(tooTall, b)
		}
	}
	return unplaced, tooTall
}

func shelvesWhereHeld(plans []ShelfPlan, starts map[int64]*db.CopyShelving, oversize bool) []int {
	var out []int
	for i, p := range plans {
		if _, ok := starts[p.ID]; ok && p.Oversize == oversize {
			out = append(out, i)
		}
	}
	return out
}

// finish fills in each shelf's first and last call numbers, and how far its books run past
// what it holds: the far side, or the brace there when the last book is too tall for it.
func finish(plans []ShelfPlan) {
	for i := range plans {
		p := &plans[i]
		if p.Books == nil {
			p.Books = []ShelfBook{}
		}
		if n := len(p.Books); n > 0 {
			p.First, p.Last = p.Books[0].CallNumber, p.Books[n-1].CallNumber
			p.OverMM = max(0, p.UsedMM-p.endFor(p.Books[n-1]))
		} else {
			p.First, p.Last = []string{}, []string{}
		}
	}
}

func startOf(b ShelfBook) string {
	j, _ := json.Marshal(b.key)
	return string(j)
}

func parseStart(s string) *db.CopyShelving {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var k db.CopyShelving
	if json.Unmarshal([]byte(s), &k) != nil {
		return nil
	}
	return &k
}
