package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/config"
	"homelibrary/internal/db"
	"homelibrary/internal/events"
)

const host = "localhost:5173"

func newApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Origins:  []string{"http://" + host},
		DBPath:   filepath.Join(dir, "test.sqlite"),
		MediaDir: filepath.Join(dir, "media"),
		DocsDir:  filepath.Join(dir, "documents"),
	}
	a, err := New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// client is one browser: it keeps the session cookie the server last set.
type client struct {
	t      *testing.T
	a      *App
	cookie string
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	r := httptest.NewRequest(method, "http://"+host+path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if c.cookie != "" {
		r.AddCookie(&http.Cookie{Name: "hl_session", Value: c.cookie})
	}
	w := httptest.NewRecorder()
	c.a.Handler().ServeHTTP(w, r)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "hl_session" {
			c.cookie = ck.Value
		}
	}
	return w
}

func (c *client) expect(method, path, body string, status int) *httptest.ResponseRecorder {
	c.t.Helper()
	w := c.do(method, path, body)
	if w.Code != status {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	return v
}

func owner(t *testing.T, a *App) *client {
	t.Helper()
	ctx := context.Background()
	u, err := auth.EnsureOwner(ctx, db.New(a.db.Write), "James")
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.NewSessionToken(ctx, a.sessions, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, a: a, cookie: token}
}

// scope is a link's choices as the share dialog sends them: every list there, empty or not.
func scope(books string, wishlist bool, papers string) catalog.Scope {
	return catalog.Scope{
		Books: books, Wishlist: wishlist, Papers: papers,
		Categories: []string{}, BookIDs: []int64{}, CourseIDs: []int64{}, PaperIDs: []int64{},
	}
}

// shareBody is a link to make or change, as JSON.
func shareBody(t *testing.T, name string, sc catalog.Scope) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"name": name, "expiresAt": "", "scope": sc})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestShareLinks(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	id, err := a.catalog.Create(context.Background(),
		catalog.BookInput{Title: "The Hobbit", Notes: "a gift from Dad", Genres: []string{"Fantasy"}},
		&catalog.CopyInput{Section: "Fiction", AcquiredAs: "used", AcquiredOn: "2024-05-01", AcquiredFrom: "Powell's",
			PriceCents: 1250, Condition: "good", DustJacket: "yes", Location: "Hall closet", Notes: "spine creased"}, "")
	if err != nil {
		t.Fatal(err)
	}
	book := "/api/v1/books/" + strconv.FormatInt(id, 10)
	copyID := decode[catalog.Book](t, me.expect(http.MethodGet, book, "", http.StatusOK)).Copies[0].ID
	copyPath := strconv.FormatInt(copyID, 10)
	me.expect(http.MethodPost, "/api/v1/copies/"+copyPath+"/loans", `{"borrower":"Rosa","lentOn":"2026-10-01","dueOn":"2026-10-15","returnedOn":"","notes":"for her trip"}`, http.StatusCreated)
	course := decode[catalog.CourseDetail](t, me.expect(http.MethodPost, "/api/v1/courses",
		`{"school":"State","level":"undergraduate","department":"ENGL","number":"210","title":"Fantasy Literature","term":"fall","year":2004,"instructor":"Dr. Ruiz","notes":""}`, http.StatusCreated))
	me.expect(http.MethodPut, "/api/v1/courses/"+strconv.FormatInt(course.ID, 10)+"/copies/"+copyPath, `{"required":true}`, http.StatusOK)

	made := decode[struct {
		Share auth.ShareLink `json:"share"`
		Token string         `json:"token"`
	}](t, me.expect(http.MethodPost, "/api/v1/shares", shareBody(t, "Mom", scope("all", true, "none")), http.StatusCreated))
	if made.Token == "" || made.Share.Name != "Mom" || made.Share.Scope.Books != "all" {
		t.Fatalf("made %+v", made)
	}
	me.expect(http.MethodPost, "/api/v1/shares", `{"name":"Past","expiresAt":"2001-01-01T00:00:00Z","scope":{"books":"all","categories":[],"bookIds":[],"wishlist":true,"papers":"none","courseIds":[],"paperIds":[]}}`, http.StatusUnprocessableEntity)

	mom := &client{t: t, a: a}
	mom.expect(http.MethodGet, "/api/v1/books", "", http.StatusUnauthorized)
	if w := mom.expect(http.MethodGet, "/s/"+made.Token, "", http.StatusSeeOther); w.Header().Get("Location") != "/" {
		t.Fatalf("link sent the visitor to %q", w.Header().Get("Location"))
	}
	who := decode[struct{ Role, Name string }](t, mom.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK))
	if who.Role != "visitor" || who.Name != "James" {
		t.Fatalf("visitor is %+v", who)
	}

	list := decode[struct{ Books []catalog.BookRow }](t, mom.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK))
	if len(list.Books) != 1 {
		t.Fatalf("visitor sees %d books", len(list.Books))
	}
	seen := decode[catalog.Book](t, mom.expect(http.MethodGet, book, "", http.StatusOK))
	c := seen.Copies[0]
	if seen.Notes != "" || c.Notes != "" || c.AcquiredFrom != "" || c.AcquiredOn != "" || c.PriceCents != -1 ||
		c.Location != "" || c.Condition != "unknown" || c.AcquiredAs != "unknown" {
		t.Fatalf("visitor sees the owner's details: %+v", seen)
	}
	if seen.Title != "The Hobbit" || c.Section != "Fiction" || c.DustJacket != "yes" || c.Barcode == "" || !c.OnLoan ||
		strings.Join(c.CallNumber, " ") != "HOB" || len(c.Labels) != 0 {
		t.Fatalf("visitor misses the catalog: %+v", seen)
	}
	raw := mom.expect(http.MethodGet, book, "", http.StatusOK).Body.String()
	for _, secret := range []string{"Dad", "Powell", "1250", "Hall closet", "creased", "2024-05-01", "Rosa", "trip", "2026-10-15", "Ruiz", "Fantasy Literature", "ENGL"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("visitor's answer has %q: %s", secret, raw)
		}
	}

	for _, r := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/shares", ""},
		{http.MethodGet, "/api/v1/choices", ""},
		{http.MethodGet, "/api/v1/passkeys", ""},
		{http.MethodGet, "/api/v1/scans", ""},
		{http.MethodGet, "/api/v1/loans", ""},
		{http.MethodGet, "/api/v1/borrowers", ""},
		{http.MethodGet, "/api/v1/courses", ""},
		{http.MethodGet, "/api/v1/courses/" + strconv.FormatInt(course.ID, 10), ""},
		{http.MethodGet, "/api/v1/labels", ""},
		{http.MethodGet, "/api/v1/bookcases", ""},
		{http.MethodGet, "/api/v1/layout", ""},
		{http.MethodGet, "/api/v1/copies/" + copyPath + "/place", ""},
		{http.MethodPut, "/api/v1/layout/in-use", ""},
		{http.MethodGet, "/api/v1/label-sheets", ""},
		{http.MethodGet, "/api/v1/sections", ""},
		{http.MethodGet, "/api/v1/labels/spine/sheets?copies=" + copyPath, ""},
		{http.MethodPut, "/api/v1/labels/spine/" + copyPath, ""},
		{http.MethodPost, "/api/v1/copies/" + copyPath + "/loans", `{"borrower":"Mom","lentOn":"2026-10-02"}`},
		{http.MethodPut, book, `{"title":"Mine now"}`},
		{http.MethodDelete, book, ""},
		{http.MethodPost, "/api/v1/shares", shareBody(t, "Me too", scope("all", true, "all"))},
		{http.MethodPut, "/api/v1/shares/" + strconv.FormatInt(made.Share.ID, 10), shareBody(t, "Mom", scope("all", true, "all"))},
	} {
		mom.expect(r.method, r.path, r.body, http.StatusForbidden)
	}

	mine := decode[catalog.Book](t, me.expect(http.MethodGet, book, "", http.StatusOK))
	if mine.Notes != "a gift from Dad" || mine.Copies[0].PriceCents != 1250 || mine.Copies[0].Loan == nil || len(mine.Courses) != 1 {
		t.Fatalf("the owner's own view lost details: %+v", mine)
	}
	if w := me.expect(http.MethodGet, "/s/"+made.Token, "", http.StatusSeeOther); w.Header().Get("Location") != "/" {
		t.Fatalf("owner sent to %q", w.Header().Get("Location"))
	}
	labels := me.expect(http.MethodGet, "/api/v1/labels/barcode/sheets?copies="+copyPath, "", http.StatusOK)
	if labels.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(labels.Body.String(), "%PDF-") {
		t.Fatalf("label sheet answered %q: %.40s", labels.Header().Get("Content-Type"), labels.Body.String())
	}
	me.expect(http.MethodDelete, "/api/v1/labels/barcode?copies="+copyPath, "", http.StatusNoContent)
	me.expect(http.MethodGet, "/api/v1/labels/barcode/sheets?copies="+copyPath, "", http.StatusUnprocessableEntity)
	if who := decode[struct{ Role string }](t, me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "owner" {
		t.Fatalf("opening a link made the owner a %s", who.Role)
	}

	shares := decode[struct{ Shares []auth.ShareLink }](t, me.expect(http.MethodGet, "/api/v1/shares", "", http.StatusOK))
	if len(shares.Shares) != 1 || shares.Shares[0].LastUsedAt == "" {
		t.Fatalf("shares %+v", shares)
	}

	me.expect(http.MethodDelete, "/api/v1/shares/"+strconv.FormatInt(made.Share.ID, 10), "", http.StatusNoContent)
	// A link made at once takes the revoked one's number: the revoked link's visitor, who has
	// not called since, is not let in by it.
	again := decode[struct {
		Share auth.ShareLink `json:"share"`
	}](t, me.expect(http.MethodPost, "/api/v1/shares", shareBody(t, "Someone else", scope("all", true, "none")), http.StatusCreated))
	if again.Share.ID != made.Share.ID {
		t.Logf("the new link has number %d, the revoked %d", again.Share.ID, made.Share.ID)
	}
	mom.expect(http.MethodGet, "/api/v1/books", "", http.StatusUnauthorized)
	if who := decode[struct{ Role string }](t, mom.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "none" {
		t.Fatalf("revoked visitor is %s", who.Role)
	}
	if w := mom.expect(http.MethodGet, "/s/"+made.Token, "", http.StatusSeeOther); w.Header().Get("Location") != "/link-ended" {
		t.Fatalf("revoked link sent to %q", w.Header().Get("Location"))
	}
}

func TestShareLinkEnds(t *testing.T) {
	a := newApp(t)
	ended, endedToken, err := auth.NewShare(context.Background(), a.db, "Last week", time.Now().Add(-time.Minute), scope("all", true, "none"))
	if err != nil {
		t.Fatal(err)
	}
	_, soonToken, err := auth.NewShare(context.Background(), a.db, "Soon", time.Now().Add(time.Hour), scope("all", true, "none"))
	if err != nil {
		t.Fatal(err)
	}
	if ended.ExpiresAt == "" {
		t.Fatal("end date not kept")
	}
	visitor := &client{t: t, a: a}
	if w := visitor.expect(http.MethodGet, "/s/"+endedToken, "", http.StatusSeeOther); w.Header().Get("Location") != "/link-ended" {
		t.Fatalf("ended link sent to %q", w.Header().Get("Location"))
	}
	visitor.expect(http.MethodGet, "/api/v1/books", "", http.StatusUnauthorized)
	visitor.expect(http.MethodGet, "/s/"+soonToken, "", http.StatusSeeOther)
	visitor.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK)
	visitor.expect(http.MethodGet, "/s/not-a-token", "", http.StatusSeeOther)
}

func TestSharedDocuments(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	doc := func(title string) catalog.Document {
		body, _ := json.Marshal(catalog.DocumentInput{Kind: "handout", Title: title, Authors: []string{}, Notes: "only mine"})
		return decode[catalog.Document](t, me.expect(http.MethodPost, "/api/v1/documents", string(body), http.StatusCreated))
	}
	upload := func(d catalog.Document) catalog.Document {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		part, _ := mw.CreateFormFile("file", "Handout.pdf")
		_, _ = part.Write([]byte("%PDF-1.4\n%%EOF\n"))
		_ = mw.Close()
		r := httptest.NewRequest(http.MethodPost, "http://"+host+"/api/v1/documents/"+strconv.FormatInt(d.ID, 10)+"/files", &body)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.AddCookie(&http.Cookie{Name: "hl_session", Value: me.cookie})
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("upload: %d %s", w.Code, w.Body.String())
		}
		return decode[catalog.Document](t, w)
	}
	private := upload(doc("Private handout"))
	shared := upload(doc("Shared reading"))
	file := func(d catalog.Document) string {
		return "/api/v1/documents/" + strconv.FormatInt(d.ID, 10) + "/files/" + strconv.FormatInt(d.Files[0].ID, 10)
	}
	if w := me.expect(http.MethodGet, file(private), "", http.StatusOK); w.Header().Get("Content-Type") != "application/pdf" ||
		!strings.HasPrefix(w.Body.String(), "%PDF") || !strings.Contains(w.Header().Get("Content-Disposition"), "Handout.pdf") {
		t.Fatalf("the owner's file came back as %q %q", w.Header().Get("Content-Type"), w.Body.String())
	}

	papers := scope("none", false, "some")
	papers.PaperIDs = []int64{shared.ID}
	made := decode[struct {
		Token string `json:"token"`
	}](t, me.expect(http.MethodPost, "/api/v1/shares", shareBody(t, "Class", papers), http.StatusCreated))
	class := &client{t: t, a: a}
	class.expect(http.MethodGet, "/s/"+made.Token, "", http.StatusSeeOther)

	list := decode[struct{ Documents []catalog.DocumentRow }](t, class.expect(http.MethodGet, "/api/v1/documents", "", http.StatusOK))
	if len(list.Documents) != 1 || list.Documents[0].ID != shared.ID {
		t.Fatalf("a visitor's list %+v", list.Documents)
	}
	if raw := class.expect(http.MethodGet, "/api/v1/documents/"+strconv.FormatInt(shared.ID, 10), "", http.StatusOK).Body.String(); strings.Contains(raw, "only mine") {
		t.Fatalf("a visitor sees the notes: %s", raw)
	}
	class.expect(http.MethodGet, file(shared), "", http.StatusOK)
	// A visitor's search reaches what they may read, never the owner's notes.
	found := decode[struct{ Documents []catalog.DocumentRow }](t, class.expect(http.MethodGet, "/api/v1/documents?q=mine", "", http.StatusOK))
	if len(found.Documents) != 0 {
		t.Fatalf("a visitor's search reached the notes: %+v", found.Documents)
	}
	if mine := decode[struct{ Documents []catalog.DocumentRow }](t, me.expect(http.MethodGet, "/api/v1/documents?q=mine", "", http.StatusOK)); len(mine.Documents) != 2 {
		t.Fatalf("the owner's search %+v", mine.Documents)
	}
	if byTitle := decode[struct{ Documents []catalog.DocumentRow }](t, class.expect(http.MethodGet, "/api/v1/documents?q=reading", "", http.StatusOK)); len(byTitle.Documents) != 1 {
		t.Fatalf("a visitor's search by title %+v", byTitle.Documents)
	}
	class.expect(http.MethodGet, "/api/v1/documents/"+strconv.FormatInt(private.ID, 10), "", http.StatusNotFound)
	class.expect(http.MethodGet, file(private), "", http.StatusNotFound)
	class.expect(http.MethodPost, "/api/v1/documents", `{"kind":"other","title":"Mine"}`, http.StatusForbidden)
	class.expect(http.MethodDelete, file(shared), "", http.StatusForbidden)
	class.expect(http.MethodGet, "/api/v1/changes", "", http.StatusForbidden)
	class.expect(http.MethodPost, "/api/v1/lookup/doi", `{"doi":"10.1000/x","document":{"kind":"other","title":""}}`, http.StatusForbidden)
	class.expect(http.MethodPost, "/api/v1/lookup/citation", `{"ris":"TY  - JOUR\nER  - ","document":{"kind":"other","title":""}}`, http.StatusForbidden)
}

// A link shows what the owner chose of it, and a change to it holds from the visitor's next
// request: the books of a category and one book besides, then only the papers of a course.
func TestShareScope(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	ctx := context.Background()
	mk := func(in catalog.BookInput, first *catalog.CopyInput) int64 {
		t.Helper()
		in.ReadStatus = "unread"
		id, err := a.catalog.Create(ctx, in, first, "")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	here := &catalog.CopyInput{PriceCents: -1}
	manga := mk(catalog.BookInput{Title: "Witch Hat Atelier 1", Category: "manga"}, here)
	novel := mk(catalog.BookInput{Title: "Blood Song", Category: "fiction"}, here)
	study := mk(catalog.BookInput{Title: "The Structure of Scientific Revolutions", Category: "nonfiction"}, here)
	mk(catalog.BookInput{Title: "Witch Hat Atelier 2", Category: "manga"}, nil)
	course := decode[catalog.CourseDetail](t, me.expect(http.MethodPost, "/api/v1/courses",
		`{"school":"State","level":"undergraduate","department":"HPS","number":"101","title":"Scientific Revolutions","term":"fall","year":2004,"instructor":"","notes":""}`, http.StatusCreated))
	paper := func(title string, courseID int64) int64 {
		body, _ := json.Marshal(catalog.DocumentInput{Kind: "article", Title: title, Authors: []string{}, CourseID: courseID})
		return decode[catalog.Document](t, me.expect(http.MethodPost, "/api/v1/documents", string(body), http.StatusCreated)).ID
	}
	ofCourse := paper("Paradigms Lost", course.ID)
	other := paper("A Paper of No Course", 0)

	sc := scope("some", false, "none")
	sc.Categories, sc.BookIDs = []string{"manga"}, []int64{study}
	made := decode[struct {
		Share auth.ShareLink `json:"share"`
		Token string         `json:"token"`
	}](t, me.expect(http.MethodPost, "/api/v1/shares", shareBody(t, "Manga club", sc), http.StatusCreated))
	if got := made.Share.Scope; got.Books != "some" || len(got.Categories) != 1 || len(got.BookIDs) != 1 || got.Wishlist {
		t.Fatalf("the link keeps %+v", got)
	}
	club := &client{t: t, a: a}
	club.expect(http.MethodGet, "/s/"+made.Token, "", http.StatusSeeOther)

	who := decode[struct {
		Role string
		Sees catalog.Sees
	}](t, club.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK))
	if who.Role != "visitor" || !who.Sees.Books || who.Sees.Wishlist || who.Sees.Papers {
		t.Fatalf("the visitor sees %+v", who)
	}
	list := decode[struct{ Books []catalog.BookRow }](t, club.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK))
	if len(list.Books) != 2 || !slices.ContainsFunc(list.Books, func(r catalog.BookRow) bool { return r.ID == manga }) ||
		!slices.ContainsFunc(list.Books, func(r catalog.BookRow) bool { return r.ID == study }) {
		t.Fatalf("the visitor's list %+v", list.Books)
	}
	book := func(id int64) string { return "/api/v1/books/" + strconv.FormatInt(id, 10) }
	club.expect(http.MethodGet, book(manga), "", http.StatusOK)
	club.expect(http.MethodGet, book(novel), "", http.StatusNotFound)
	if docs := decode[struct{ Documents []catalog.DocumentRow }](t, club.expect(http.MethodGet, "/api/v1/documents", "", http.StatusOK)); len(docs.Documents) != 0 {
		t.Fatalf("papers through a link of books: %+v", docs.Documents)
	}
	if mine := decode[struct{ Books []catalog.BookRow }](t, me.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK)); len(mine.Books) != 4 {
		t.Fatalf("the owner's list %d", len(mine.Books))
	}

	// Changed to a course's papers and no books: the visitor's next request sees that.
	link := "/api/v1/shares/" + strconv.FormatInt(made.Share.ID, 10)
	me.expect(http.MethodPut, link, shareBody(t, "Manga club", scope("some", false, "none")), http.StatusUnprocessableEntity)
	me.expect(http.MethodPut, link, shareBody(t, "Manga club", scope("none", true, "none")), http.StatusUnprocessableEntity)
	me.expect(http.MethodPut, "/api/v1/shares/999", shareBody(t, "No one", scope("all", true, "all")), http.StatusNotFound)
	byCourse := scope("none", true, "some")
	byCourse.CourseIDs = []int64{course.ID}
	changed := decode[auth.ShareLink](t, me.expect(http.MethodPut, link, shareBody(t, "HPS 101", byCourse), http.StatusOK))
	if changed.Name != "HPS 101" || changed.Scope.Books != "none" || changed.Scope.Wishlist || len(changed.Scope.CourseIDs) != 1 {
		t.Fatalf("changed %+v", changed)
	}
	if list := decode[struct{ Books []catalog.BookRow }](t, club.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK)); len(list.Books) != 0 {
		t.Fatalf("books through a link of papers: %+v", list.Books)
	}
	club.expect(http.MethodGet, book(manga), "", http.StatusNotFound)
	docs := decode[struct{ Documents []catalog.DocumentRow }](t, club.expect(http.MethodGet, "/api/v1/documents", "", http.StatusOK))
	if len(docs.Documents) != 1 || docs.Documents[0].ID != ofCourse || docs.Documents[0].Course != "" {
		t.Fatalf("the course's papers %+v", docs.Documents)
	}
	paperPath := func(id int64) string { return "/api/v1/documents/" + strconv.FormatInt(id, 10) }
	club.expect(http.MethodGet, paperPath(ofCourse), "", http.StatusOK)
	club.expect(http.MethodGet, paperPath(other), "", http.StatusNotFound)
	if who := decode[struct{ Sees catalog.Sees }](t, club.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Sees.Books || !who.Sees.Papers {
		t.Fatalf("the visitor now sees %+v", who.Sees)
	}
	if raw := club.expect(http.MethodGet, "/api/v1/series", "", http.StatusOK).Body.String(); strings.Contains(raw, "Witch") {
		t.Fatalf("series through a link of papers: %s", raw)
	}
	// The course removed, the link shows none of its papers.
	me.expect(http.MethodDelete, "/api/v1/courses/"+strconv.FormatInt(course.ID, 10), "", http.StatusNoContent)
	if shares := decode[struct{ Shares []auth.ShareLink }](t, me.expect(http.MethodGet, "/api/v1/shares", "", http.StatusOK)); len(shares.Shares[0].Scope.CourseIDs) != 0 {
		t.Fatalf("a removed course is still chosen: %+v", shares.Shares[0].Scope)
	}
}

func TestCalendarLink(t *testing.T) {
	a := newApp(t)
	o := owner(t, a)
	ctx := context.Background()
	id, err := a.catalog.Create(ctx, catalog.BookInput{Title: "Crowfall, a Raven's Mark novel; book three", ReadStatus: "unread"}, &catalog.CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := a.catalog.Get(ctx, id)
	if _, err := a.catalog.Lend(ctx, b.Copies[0].ID, catalog.LoanInput{Borrower: "Sam", LentOn: "2026-10-01", DueOn: "2026-10-20"}); err != nil {
		t.Fatal(err)
	}

	stranger := &client{t: t, a: a}
	stranger.expect(http.MethodGet, "/calendar/guess.ics", "", http.StatusNotFound)
	stranger.expect(http.MethodPost, "/api/v1/calendar-link", "", http.StatusUnauthorized)

	made := decode[struct{ Token string }](t, o.expect(http.MethodPost, "/api/v1/calendar-link", "", http.StatusCreated))
	w := stranger.expect(http.MethodGet, "/calendar/"+made.Token+".ics", "", http.StatusOK)
	feed := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("content type %q", ct)
	}
	for _, want := range []string{"BEGIN:VCALENDAR\r\n", "DTSTART;VALUE=DATE:20261020\r\n", "DTEND;VALUE=DATE:20261021\r\n", `Raven's Mark novel\; book three`, "END:VCALENDAR\r\n"} {
		if !strings.Contains(strings.ReplaceAll(feed, "\r\n ", ""), want) {
			t.Errorf("the feed lacks %q:\n%s", want, feed)
		}
	}
	for _, line := range strings.Split(feed, "\r\n") {
		if len(line) > 75 {
			t.Errorf("a line of %d octets: %q", len(line), line)
		}
	}
	if got := decode[auth.CalendarLink](t, o.expect(http.MethodGet, "/api/v1/calendar-link", "", http.StatusOK)); !got.Made || got.LastUsedAt == "" {
		t.Errorf("the link as listed %+v", got)
	}

	// A new link replaces the old; turned off, there is none.
	again := decode[struct{ Token string }](t, o.expect(http.MethodPost, "/api/v1/calendar-link", "", http.StatusCreated))
	stranger.expect(http.MethodGet, "/calendar/"+made.Token+".ics", "", http.StatusNotFound)
	o.expect(http.MethodDelete, "/api/v1/calendar-link", "", http.StatusNoContent)
	stranger.expect(http.MethodGet, "/calendar/"+again.Token+".ics", "", http.StatusNotFound)
}

// The owner's stream tells of a change as it is made: a book added names the book, and its
// copy's label waiting to print.
func TestChangeStream(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	a.feed.Poll = 20 * time.Millisecond
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/changes", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	req.AddCookie(&http.Cookie{Name: "hl_session", Value: me.cookie})
	go func() {
		time.Sleep(200 * time.Millisecond) // the stream has its cursor
		if _, err := a.catalog.Create(context.Background(), catalog.BookInput{Title: "Blood Song", ReadStatus: "unread"},
			&catalog.CopyInput{PriceCents: -1}, ""); err != nil {
			t.Error(err)
		}
	}()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("status %d, type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	seen := map[string]bool{}
	lines := bufio.NewScanner(resp.Body)
	for !seen["book"] || !seen["label"] {
		if !lines.Scan() {
			t.Fatalf("the stream ended having told of %v: %v", seen, lines.Err())
		}
		if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
			var c events.Change
			if err := json.Unmarshal([]byte(data), &c); err != nil {
				t.Fatal(err)
			}
			seen[c.Type] = true
		}
	}
}
