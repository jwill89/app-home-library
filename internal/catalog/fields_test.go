package catalog

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// The fields a lookup names are one list: each enum the frontend is typed from says the same,
// and the declined suggestions' table takes every one.
func TestLookupFieldsAreTheSuggestionsEnum(t *testing.T) {
	for _, of := range []reflect.Type{reflect.TypeFor[Suggestions](), reflect.TypeFor[SuggestedBook](), reflect.TypeFor[Declined]()} {
		f, _ := of.FieldByName("Fields")
		if got, want := f.Tag.Get("enum"), strings.Join(lookupFields, ","); got != want {
			t.Errorf("%s.Fields enum %q, lookupFields %q", of.Name(), got, want)
		}
	}
	if f, _ := reflect.TypeFor[Declined]().FieldByName("Fields"); f.Tag.Get("maxItems") != strconv.Itoa(len(lookupFields)) {
		t.Errorf("Declined.Fields maxItems %q, %d fields", f.Tag.Get("maxItems"), len(lookupFields))
	}
	ctx := context.Background()
	s := newService(t)
	id, err := s.Create(ctx, BookInput{Title: "Any", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	q := db.New(s.DB.Write)
	for _, f := range lookupFields {
		if err := q.DeclineSuggestion(ctx, db.DeclineSuggestionParams{EditionID: id, Field: f, Value: `""`}); err != nil {
			t.Errorf("declining %s: %v", f, err)
		}
	}
}

// Every field an import fills is the app's, so a lookup leaves it be: place, edition, and the
// measurements as much as the title.
func TestImportedMarksEveryFieldItFilled(t *testing.T) {
	in := BookInput{Title: "Leviathan", PublishPlace: "Indianapolis", EditionStatement: "2nd ed.", WidthMM: 140,
		ThicknessMM: 30, Subjects: []string{"Political science"}, Category: "nonfiction", Sources: map[string]string{}}
	imported(&in, "bookshelf")
	for _, f := range []string{"title", "publishPlace", "editionStatement", "widthMm", "thicknessMm", "subjects", "category"} {
		if in.Sources[f] != "bookshelf" {
			t.Errorf("%s: source %q", f, in.Sources[f])
		}
	}
	for _, f := range []string{"subtitle", "pages", "heightMm", "contributors"} {
		if _, ok := in.Sources[f]; ok {
			t.Errorf("%s was left blank, yet marked", f)
		}
	}
}

// Each list of values the server takes is written twice, as a variable the code reads and as
// the enum the API (and the frontend's types) are made from: the two say the same.
func TestValueListsAreTheirEnums(t *testing.T) {
	enum := func(of reflect.Type, field string) string {
		f, ok := of.FieldByName(field)
		if !ok {
			t.Fatalf("%s has no field %s", of.Name(), field)
		}
		return f.Tag.Get("enum")
	}
	for _, c := range []struct {
		name string
		list []string
		tag  string
	}{
		{"Categories (as set)", append([]string{""}, Categories...), enum(reflect.TypeFor[BookInput](), "Category")},
		{"Categories (as shown)", Categories, enum(reflect.TypeFor[Book](), "Category")},
		{"Categories (as shared)", Categories, enum(reflect.TypeFor[Scope](), "Categories")},
		{"Formats (as set)", append([]string{""}, lookup.Formats...), enum(reflect.TypeFor[BookInput](), "Format")},
		{"Formats (as shown)", append([]string{""}, lookup.Formats...), enum(reflect.TypeFor[Book](), "Format")},
		{"Formats (as found)", append([]string{""}, lookup.Formats...), enum(reflect.TypeFor[lookup.Proposal](), "Format")},
		{"Roles", lookup.Roles, enum(reflect.TypeFor[lookup.Contributor](), "Role")},
		{"documentKinds", documentKinds, enum(reflect.TypeFor[DocumentInput](), "Kind")},
		{"GapFields (missing)", GapFields, enum(reflect.TypeFor[BookGaps](), "Missing")},
	} {
		if got := strings.Join(c.list, ","); got != c.tag {
			t.Errorf("%s: list %q, enum %q", c.name, got, c.tag)
		}
	}
}
