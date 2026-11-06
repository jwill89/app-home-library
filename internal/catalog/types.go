// Package catalog holds the library's rules: what a book, an edition, and a copy are, how
// one is added and changed, and how a lookup fills the blanks an edition has.
package catalog

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"homelibrary/internal/lookup"
)

// subjectLimit is the most subjects a book takes, as BookInput's maxItems says: a lookup's
// merge keeps to it, so it never cuts a list the owner made.
const subjectLimit = 40

// BookInput is a book as the owner adds or edits it: the work, the edition, and, when
// adding, the first copy.
type BookInput struct {
	Title        string               `json:"title" minLength:"1" maxLength:"500"`
	Subtitle     string               `json:"subtitle" maxLength:"500"`
	Contributors []lookup.Contributor `json:"contributors" maxItems:"40"`
	Series       string               `json:"series" maxLength:"300"`
	SeriesNumber string               `json:"seriesNumber" maxLength:"20"`
	Category     string               `json:"category" enum:",fiction,nonfiction,manga,light_novel,graphic_novel,uncategorized" doc:"Empty to have it worked out from the book's details"`
	ReadStatus   string               `json:"readStatus" enum:"unread,reading,read"`
	CurrentPage  int64                `json:"currentPage" minimum:"0" doc:"Page reached, while reading; 0 for none"`
	StartedOn    string               `json:"startedOn" doc:"YYYY-MM-DD, or empty"`
	FinishedOn   string               `json:"finishedOn" doc:"YYYY-MM-DD, or empty"`
	Rating       int64                `json:"rating" minimum:"0" maximum:"5" doc:"1 to 5; 0 for none"`
	Notes        string               `json:"notes" maxLength:"20000"`
	Genres       []string             `json:"genres" maxItems:"30"`

	ISBN             string            `json:"isbn" maxLength:"20" doc:"ISBN-10 or ISBN-13; empty for a book without one"`
	LCCN             string            `json:"lccn" maxLength:"40"`
	Publisher        string            `json:"publisher" maxLength:"300"`
	PublishPlace     string            `json:"publishPlace" maxLength:"200"`
	Published        string            `json:"published" maxLength:"10" doc:"YYYY, YYYY-MM, or YYYY-MM-DD"`
	EditionStatement string            `json:"editionStatement" maxLength:"200"`
	Format           string            `json:"format" enum:",hardcover,paperback,trade_paperback,mass_market,library_binding,spiral,board_book,leather,other" doc:"Empty when not known"`
	Pages            int64             `json:"pages" minimum:"0"`
	HeightMM         int64             `json:"heightMm" minimum:"0" maximum:"2000"`
	WidthMM          int64             `json:"widthMm" minimum:"0" maximum:"2000"`
	ThicknessMM      int64             `json:"thicknessMm" minimum:"0" maximum:"1000"`
	Language         string            `json:"language" maxLength:"10"`
	DDC              string            `json:"ddc" maxLength:"30"`
	LCC              string            `json:"lcc" maxLength:"60"`
	Subjects         []string          `json:"subjects" maxItems:"40"`
	Description      string            `json:"description" maxLength:"20000"`
	Sources          map[string]string `json:"sources" doc:"Where each field came from; 'owner' for what was typed"`
}

// CopyInput is one physical copy as the owner records it.
type CopyInput struct {
	Section      string `json:"section" maxLength:"100"`
	AcquiredAs   string `json:"acquiredAs" enum:"unknown,new,used"`
	AcquiredOn   string `json:"acquiredOn" doc:"YYYY-MM-DD, or empty"`
	AcquiredFrom string `json:"acquiredFrom" maxLength:"200"`
	PriceCents   int64  `json:"priceCents" minimum:"-1" doc:"What was paid, in cents; -1 when not known"`
	Condition    string `json:"condition" enum:"unknown,fine,very_good,good,fair,poor"`
	DustJacket   string `json:"dustJacket" enum:"unknown,yes,no"`
	Signed       bool   `json:"signed"`
	Location     string `json:"location" maxLength:"200"`
	Notes        string `json:"notes" maxLength:"5000"`
}

// Book is an edition with its work and copies, as the book's page shows it.
type Book struct {
	ID               int64                `json:"id"`
	WorkID           int64                `json:"workId"`
	Title            string               `json:"title"`
	Subtitle         string               `json:"subtitle"`
	Contributors     []lookup.Contributor `json:"contributors"`
	Series           string               `json:"series"`
	SeriesID         int64                `json:"seriesId" doc:"0 for a book in no series"`
	SeriesNumber     string               `json:"seriesNumber"`
	Category         string               `json:"category" enum:"fiction,nonfiction,manga,light_novel,graphic_novel,uncategorized" doc:"As set, or worked out from its details, which its sources then name as inferred"`
	ReadStatus       string               `json:"readStatus"`
	CurrentPage      int64                `json:"currentPage"`
	StartedOn        string               `json:"startedOn"`
	FinishedOn       string               `json:"finishedOn"`
	Rating           int64                `json:"rating"`
	Notes            string               `json:"notes"`
	Genres           []string             `json:"genres"`
	ISBN13           string               `json:"isbn13"`
	ISBN10           string               `json:"isbn10"`
	LCCN             string               `json:"lccn"`
	Publisher        string               `json:"publisher"`
	PublishPlace     string               `json:"publishPlace"`
	Published        string               `json:"published"`
	EditionStatement string               `json:"editionStatement"`
	Format           string               `json:"format" enum:",hardcover,paperback,trade_paperback,mass_market,library_binding,spiral,board_book,leather,other"`
	Pages            int64                `json:"pages"`
	HeightMM         int64                `json:"heightMm"`
	WidthMM          int64                `json:"widthMm"`
	ThicknessMM      int64                `json:"thicknessMm"`
	Language         string               `json:"language"`
	DDC              string               `json:"ddc"`
	LCC              string               `json:"lcc"`
	Subjects         []string             `json:"subjects"`
	Description      string               `json:"description"`
	DescriptionCut   bool                 `json:"descriptionCut" doc:"The description is the Bookshelf export's, cut off, and no source had it whole"`
	Cover            Cover                `json:"cover"`
	Sources          map[string]string    `json:"sources"`
	Copies           []Copy               `json:"copies"`
	Courses          []CourseUse          `json:"courses" doc:"The courses its copies were used for; the owner's alone"`
	Links            []Link               `json:"links" doc:"Where the book is in the apps it was imported from; the owner's alone"`
	Enrichment       string               `json:"enrichment" doc:"The background lookup: pending, done, failed, or empty"`
	WantedOn         string               `json:"wantedOn" doc:"The day it went on the wishlist; empty for a book not wanted"`
	Readings         []Reading            `json:"readings" doc:"Earlier reads, newest first; the read in progress or the latest is the work's own"`
	CreatedAt        string               `json:"createdAt"`
	UpdatedAt        string               `json:"updatedAt"`
}

// Link is where a book is in another app it was imported from.
type Link struct {
	Source string `json:"source" enum:"goodreads,librarything"`
	URL    string `json:"url"`
}

// Cover names a stored cover and the widths it has. An empty ID is no cover.
type Cover struct {
	ID     string `json:"id"`
	Widths []int  `json:"widths"`
}

// Copy is one physical copy, and the loan it is out on, if any.
type Copy struct {
	ID      int64  `json:"id"`
	Barcode string `json:"barcode"`
	CopyInput
	CallNumber []string `json:"callNumber" doc:"The spine label's lines: the class, the Cutter, the volume"`
	Labels     []string `json:"labels" doc:"Its labels waiting to be printed: barcode, spine; the owner's alone"`
	OnLoan     bool     `json:"onLoan"`
	GoneAs     string   `json:"goneAs" enum:",lost,sold,donated,given,discarded" doc:"How it left the library; empty while it is here"`
	GoneOn     string   `json:"goneOn" doc:"YYYY-MM-DD it went, or empty"`
	GoneNote   string   `json:"goneNote"`
	SeenOn     string   `json:"seenOn" doc:"YYYY-MM-DD a shelf check last found it, or empty"`
	Loan       *Loan    `json:"loan,omitempty" doc:"Who has it; the owner's alone"`
	CreatedAt  string   `json:"createdAt"`
}

// ForVisitor is the book as a share link shows it: the catalog and the reading, and whether a
// copy is on the shelf, without the owner's notes, the courses, who has a copy, or anything
// about what a copy cost, where it came from, or its state.
func (b *Book) ForVisitor() *Book {
	v := *b
	v.Notes, v.Enrichment = "", ""
	v.Courses = []CourseUse{}
	v.Links = []Link{}
	// A visitor sees the copies in the house: one that has gone is the owner's record alone.
	v.Copies = make([]Copy, 0, len(b.Copies))
	for _, c := range b.Copies {
		if c.GoneAs != "" {
			continue
		}
		v.Copies = append(v.Copies, Copy{ID: c.ID, Barcode: c.Barcode, CreatedAt: c.CreatedAt, OnLoan: c.OnLoan, CallNumber: c.CallNumber, Labels: []string{}, CopyInput: CopyInput{
			Section:    c.Section,
			AcquiredAs: "unknown",
			PriceCents: -1,
			Condition:  "unknown",
			DustJacket: c.DustJacket,
			Signed:     c.Signed,
		}})
	}
	return &v
}

// BookRow is one edition in the library list.
type BookRow struct {
	ID           int64  `json:"id"`
	WorkID       int64  `json:"workId"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle"`
	Authors      string `json:"authors"`
	Series       string `json:"series"`
	SeriesID     int64  `json:"seriesId" doc:"0 for a book in no series"`
	SeriesNumber string `json:"seriesNumber"`
	Category     string `json:"category" doc:"As set, or worked out from its details"`
	ISBN13       string `json:"isbn13"`
	Format       string `json:"format"`
	DDC          string `json:"ddc"`
	Published    string `json:"published"`
	Publisher    string `json:"publisher"`
	ReadStatus   string `json:"readStatus"`
	Sections     string `json:"sections"`
	Cover        Cover  `json:"cover"`
	Copies       int64  `json:"copies"`
	WantedOn     string `json:"wantedOn" doc:"The day it went on the wishlist; empty for a book not wanted"`
	HadAs        int64  `json:"hadAs" doc:"For a wanted book, an edition of the same work the house has; 0 when it has none"`
	Barcode      string `json:"barcode" doc:"The first copy's"`
	AddedAt      string `json:"addedAt"`
	// What else the search looks in.
	Subjects []string `json:"subjects"`
	Genres   string   `json:"genres"`
	Credits  string   `json:"credits" doc:"Everyone credited besides the authors: editors, translators, illustrators, narrators"`
	LCC      string   `json:"lcc"`
}

// Barcode is a copy's label: HL and the copy number, at least five digits.
func Barcode(copyID int64) string {
	return fmt.Sprintf("HL%05d", copyID)
}

// ParseBarcode reads a scanned label back into a copy number.
func ParseBarcode(s string) (int64, bool) {
	s = strings.ToUpper(strings.TrimSpace(s))
	digits, ok := strings.CutPrefix(s, "HL")
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(digits, 10, 64)
	return id, err == nil && id > 0
}

var (
	// ErrNotFound is a book, copy, or cover that is not there.
	ErrNotFound = errors.New("not found")
)

// DuplicateError is an ISBN the library already has.
type DuplicateError struct {
	ID int64
}

func (e *DuplicateError) Error() string {
	return fmt.Sprintf("the library already has this ISBN (book %d)", e.ID)
}

// ConflictError is a change the library's state refuses, said as the owner should read it.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}

// FieldError is input refused for one field.
type FieldError struct {
	Field   string
	Message string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Message
}

var (
	dayRe       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	publishedRe = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)
)

// normalize trims the input and checks what the schema cannot, returning the ISBN-13.
func (in *BookInput) normalize() (isbn13 string, err error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return "", &FieldError{"title", "a book needs a title"}
	}
	if s := strings.TrimSpace(in.ISBN); s != "" {
		n, ok := lookup.NormalizeISBN(s)
		if !ok {
			return "", &FieldError{"isbn", "not a valid ISBN; check the digits"}
		}
		isbn13 = n
	}
	if in.Category != "" && !slices.Contains(Categories, in.Category) {
		return "", &FieldError{"category", "not a category the library has"}
	}
	if in.ReadStatus == "" {
		in.ReadStatus = "unread"
	}
	if in.Format != "" && !slices.Contains(lookup.Formats, in.Format) {
		return "", &FieldError{"format", "not a format the library knows"}
	}
	if in.Published = strings.TrimSpace(in.Published); in.Published != "" && !publishedRe.MatchString(in.Published) {
		return "", &FieldError{"published", "write it as YYYY, YYYY-MM, or YYYY-MM-DD"}
	}
	for field, v := range map[string]string{"startedOn": in.StartedOn, "finishedOn": in.FinishedOn} {
		if v != "" && !dayRe.MatchString(v) {
			return "", &FieldError{field, "write it as YYYY-MM-DD"}
		}
	}
	var people []lookup.Contributor
	for _, c := range in.Contributors {
		c.Name = strings.TrimSpace(c.Name)
		if c.Name == "" {
			continue
		}
		if !slices.Contains(lookup.Roles, c.Role) {
			return "", &FieldError{"contributors", "not a role the library knows: " + c.Role}
		}
		people = append(people, c)
	}
	in.Contributors = people
	in.Genres = cleanList(in.Genres)
	in.Subjects = cleanList(in.Subjects)
	if d := strings.TrimSpace(in.DDC); d != "" {
		if in.DDC = lookup.NormalizeDDC(d); in.DDC == "" {
			return "", &FieldError{"ddc", "a Dewey number is three digits and an optional decimal, like 153.42"}
		}
	}
	if in.Sources == nil {
		in.Sources = map[string]string{}
	}
	return isbn13, nil
}

func (in *CopyInput) normalize() error {
	if in.AcquiredOn != "" && !dayRe.MatchString(in.AcquiredOn) {
		return &FieldError{"acquiredOn", "write it as YYYY-MM-DD"}
	}
	in.Section = strings.TrimSpace(in.Section)
	return nil
}

// cleanList trims a list of names and drops blanks and repeats, keeping the first spelling.
func cleanList(list []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			continue
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out
}
