package catalog

import (
	"context"
	"strings"
	"testing"

	"homelibrary/internal/lookup"
)

func TestInferCategory(t *testing.T) {
	for _, c := range []struct {
		ddc, series, title, publisher string
		subjects                      []string
		want                          string
	}{
		{"", "High School Prodigies Have It Easy Even in Another World! Light Novel", "Volume 2", "Seven Seas", nil, "light_novel"},
		{"741.5952", "", "Wotakoi 3", "Kodansha", nil, "manga"},
		{"", "Sweat and Soap", "Sweat and Soap 2", "Kodansha Comics", nil, "manga"},
		{"", "", "Gal Gohan 4", "", []string{"Comics & Graphic Novels / Manga / Romance"}, "manga"},
		{"741.5973", "The Witcher", "Curse of Crows", "Dark Horse Books", []string{"Graphic Novels & Manga", "Media Tie-In"}, "graphic_novel"},
		{"741.5", "Magic Artisan Dahlia Wilts No More (Manga)", "Volume 3", "Seven Seas", []string{"Graphic Novels & Manga", "Isekai"}, "manga"},
		{"813.6", "Dragon Age", "Hard in Hightown", "Dark Horse Books", []string{"Fiction"}, "fiction"},
		{"", "Saga", "Saga, Volume 1", "Image Comics", nil, "graphic_novel"},
		{"", "", "History of Philosophy, Volume 9", "Image", []string{"Philosophy", "History & Surveys"}, "nonfiction"},
		{"", "", "君のお母さんを僕に下さい! (4) (ガンガンコミックス UP!)", "スクウェア・エニックス", nil, "manga"},
		{"", "", "Causation: A User's Guide", "Oxford University Press", nil, "nonfiction"},
		{"", "", "Mass Effect: Annihilation", "Titan Books", []string{"Military Science Fiction"}, "fiction"},
		{"823.92", "", "Blood Song", "Ace", nil, "fiction"},
		{"", "", "Wrath of Empire", "Orbit", nil, "fiction"},
		{"", "", "The Name of the Wind", "", []string{"Fantasy fiction"}, "fiction"},
		{"501", "", "The Structure of Scientific Revolutions", "University of Chicago Press", nil, "nonfiction"},
		{"809.3", "", "Aspects of the Novel", "", nil, "nonfiction"},
		{"", "", "Unknown Quantity", "Plume", nil, "uncategorized"},
		{"", "", "A History", "The History Press", nil, "uncategorized"}, // not Tor
	} {
		if got := inferCategory(c.ddc, c.series, c.title, c.publisher, c.subjects); got != c.want {
			t.Errorf("%q: %q, want %q", c.title, got, c.want)
		}
	}
	in := &BookInput{Title: "The Saint's Magic Power is Omnipotent 2", Series: "The Saint's Magic Power is Omnipotent (Light Novel)"}
	if got := shelfCategory("Manga and Light Novels", in); got != "light_novel" {
		t.Errorf("a light novel on the manga shelf: %q", got)
	}
	if got := shelfCategory("Manga and Light Novels", &BookInput{Title: "Wotakoi 1"}); got != "manga" {
		t.Errorf("manga shelf: %q", got)
	}
	if got := shelfCategory("To read", in); got != "" {
		t.Errorf("another shelf: %q", got)
	}
}

func TestCategoryKeptOrWorkedOut(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", DDC: "823.92", ReadStatus: "unread",
		Contributors: []lookup.Contributor{{Name: "Anthony Ryan", Role: "author"}}}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get(ctx, id)
	if b.Category != "fiction" || b.Sources["category"] != sourceInferred {
		t.Fatalf("worked out: %q from %q", b.Category, b.Sources["category"])
	}
	// Saved as it was, it stays worked out: a new class number sorts it again.
	in := b.Input()
	in.DDC = "501"
	if err := s.Update(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if b, _ = s.Get(ctx, id); b.Category != "nonfiction" {
		t.Errorf("after a new class number: %q", b.Category)
	}
	// The owner's choice, "uncategorized" too, is kept whatever the details say.
	in = b.Input()
	in.Category, in.Sources["category"] = "uncategorized", SourceOwner
	if err := s.Update(ctx, id, in); err != nil {
		t.Fatal(err)
	}
	if b, _ = s.Get(ctx, id); b.Category != "uncategorized" || b.Sources["category"] != SourceOwner {
		t.Errorf("owner's: %q from %q", b.Category, b.Sources["category"])
	}
	rows, _ := s.List(ctx)
	if rows[0].Category != "uncategorized" {
		t.Errorf("listed as %q", rows[0].Category)
	}
	if err := s.Update(ctx, id, BookInput{Title: "X", Category: "poetry", ReadStatus: "unread"}); err == nil {
		t.Error("a category the library lacks was taken")
	}
}

// The Bookshelf export, its shelf the owner's own sorting.
const shelvedCSV = `"Book Id","ISBN","Bookshelf","Title","Authors","Publisher"
"7","","Fiction","A Shelved Novel","Ann Writer","University Press"
`

func TestImportTakesTheShelf(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	if _, err := s.ImportFile(ctx, strings.NewReader(shelvedCSV), ImportOptions{}); err != nil {
		t.Fatal(err)
	}
	rows, _ := s.List(ctx)
	if len(rows) != 1 || rows[0].Category != "fiction" {
		t.Fatalf("rows %+v", rows)
	}
	b, _ := s.Get(ctx, rows[0].ID)
	if b.Sources["category"] != ImportSourceBookshelf {
		t.Errorf("source %q", b.Sources["category"])
	}
}
