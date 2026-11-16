package server

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"homelibrary/internal/catalog"
)

// A read nobody signed in to is public: no-cache with an ETag, and a 304 with no body for a
// browser that holds it already. The owner's answers, a visitor's, and an error are private.
func TestCacheHeaders(t *testing.T) {
	a := newApp(t)
	stranger := &client{t: t, a: a}
	w := stranger.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)
	tag := w.Header().Get("ETag")
	if w.Header().Get("Cache-Control") != "no-cache" || !strings.HasPrefix(tag, `"`) {
		t.Fatalf("a public read: %v", w.Header())
	}
	r := stranger.request(http.MethodGet, "/api/v1/me", "")
	r.Header.Set("If-None-Match", tag)
	again := stranger.serve(r)
	if again.Code != http.StatusNotModified || again.Body.Len() != 0 || again.Header().Get("ETag") != tag {
		t.Fatalf("a read the browser holds: %d %v %q", again.Code, again.Header(), again.Body.String())
	}
	health := stranger.expect(http.MethodGet, "/healthz", "", http.StatusOK)
	if health.Header().Get("Cache-Control") != "no-cache" || health.Header().Get("ETag") == "" {
		t.Errorf("the health check: %v", health.Header())
	}
	missing := stranger.expect(http.MethodGet, "/api/v1/no-such-thing", "", http.StatusNotFound)
	if missing.Header().Get("Cache-Control") != "private, no-store" || missing.Header().Get("ETag") != "" {
		t.Errorf("an error: %v", missing.Header())
	}

	// A list that refuses its caller is an error like any other.
	refused := stranger.expect(http.MethodGet, "/api/v1/books", "", http.StatusUnauthorized)
	if refused.Header().Get("Cache-Control") != "private, no-store" || refused.Header().Get("ETag") != "" {
		t.Errorf("a list refused: %v", refused.Header())
	}

	me := owner(t, a)
	mine := me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)
	if mine.Header().Get("Cache-Control") != "private, no-store" || mine.Header().Get("ETag") != "" {
		t.Errorf("the owner's own answer: %v", mine.Header())
	}
}

// A list the browser receives whole is kept in its own cache alone and asked for again each
// time: an unchanged one is an empty 304, and one written to since comes back whole.
func TestKeptLists(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	first := me.expect(http.MethodGet, "/api/v1/books", "", http.StatusOK)
	tag := first.Header().Get("ETag")
	if first.Header().Get("Cache-Control") != "private, no-cache" || !strings.HasPrefix(tag, `"`) {
		t.Fatalf("the library list: %v", first.Header())
	}
	held := func() *http.Request {
		r := me.request(http.MethodGet, "/api/v1/books", "")
		r.Header.Set("If-None-Match", tag)
		return r
	}
	same := me.serve(held())
	if same.Code != http.StatusNotModified || same.Body.Len() != 0 {
		t.Fatalf("an unchanged list: %d %q", same.Code, same.Body.String())
	}

	if _, err := a.catalog.Create(context.Background(), catalog.BookInput{Title: "Blood Song", ReadStatus: "unread"},
		&catalog.CopyInput{PriceCents: -1}, ""); err != nil {
		t.Fatal(err)
	}
	fresh := me.serve(held())
	if fresh.Code != http.StatusOK || fresh.Header().Get("ETag") == tag || !strings.Contains(fresh.Body.String(), "Blood Song") {
		t.Fatalf("a list written to since: %d %v", fresh.Code, fresh.Header())
	}

	// Signing out lets go of what the browser kept.
	out := me.expect(http.MethodPost, "/api/v1/auth/logout", "", http.StatusNoContent)
	if got := out.Header().Get("Clear-Site-Data"); got != `"cache"` {
		t.Errorf("signing out: Clear-Site-Data %q", got)
	}
}

// The owner sets how much the server logs while it runs; nobody else may, and a level that
// isn't one is refused.
func TestLogLevel(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	got := decode[map[string]string](t, me.expect(http.MethodGet, "/api/v1/log-level", "", http.StatusOK))
	if got["level"] != "info" || got["started"] != "info" {
		t.Fatalf("at start: %v", got)
	}
	got = decode[map[string]string](t, me.expect(http.MethodPut, "/api/v1/log-level", `{"level":"debug"}`, http.StatusOK))
	if got["level"] != "debug" || got["started"] != "info" {
		t.Errorf("set to debug: %v", got)
	}
	if !a.logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("the logger leaves out debug lines")
	}
	me.expect(http.MethodPut, "/api/v1/log-level", `{"level":"loud"}`, http.StatusUnprocessableEntity)
	stranger := &client{t: t, a: a}
	stranger.expect(http.MethodGet, "/api/v1/log-level", "", http.StatusUnauthorized)
	stranger.expect(http.MethodPut, "/api/v1/log-level", `{"level":"error"}`, http.StatusUnauthorized)
}

// The server sets every timeout but a write timeout, and the change feed outlives the read
// timeout, which bounds reading the request alone.
func TestTheFeedOutlivesTheReadTimeout(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	a.feed.Poll = 20 * time.Millisecond
	srv := a.httpServer()
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.IdleTimeout == 0 || srv.WriteTimeout != 0 {
		t.Fatalf("timeouts %v %v %v %v", srv.ReadHeaderTimeout, srv.ReadTimeout, srv.IdleTimeout, srv.WriteTimeout)
	}
	srv.ReadTimeout = 200 * time.Millisecond
	ln, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ln.Addr().String()+"/api/v1/changes", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	req.AddCookie(&http.Cookie{Name: "hl_session", Value: me.cookie})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stream: %d", resp.StatusCode)
	}
	time.Sleep(3 * srv.ReadTimeout)
	if _, err := a.catalog.Create(context.Background(), catalog.BookInput{Title: "Blood Song", ReadStatus: "unread"},
		&catalog.CopyInput{PriceCents: -1}, ""); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(resp.Body)
	for lines.Scan() {
		if strings.Contains(lines.Text(), `"type":"book"`) {
			return
		}
	}
	t.Fatalf("the stream ended before the change: %v", lines.Err())
}

// A form's save made before the record's last change is refused 409, its error at
// body.updatedAt so the page can tell it from any other conflict; one sent with the stamp it
// read goes through.
func TestStaleSaveIsRefused(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	ctx := context.Background()
	bookID, err := a.catalog.Create(ctx, catalog.BookInput{Title: "Blood Song", ReadStatus: "unread"}, &catalog.CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.catalog.Get(ctx, bookID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.catalog.Lend(ctx, b.Copies[0].ID, catalog.LoanInput{Borrower: "Ana", LentOn: "2026-11-01"}); err != nil {
		t.Fatal(err)
	}
	type borrower struct {
		ID        int64  `json:"id"`
		UpdatedAt string `json:"updatedAt"`
	}
	read := func() borrower {
		t.Helper()
		list := decode[struct{ Borrowers []borrower }](t, me.expect(http.MethodGet, "/api/v1/borrowers", "", http.StatusOK))
		if len(list.Borrowers) != 1 || list.Borrowers[0].UpdatedAt == "" {
			t.Fatalf("borrowers %+v", list)
		}
		return list.Borrowers[0]
	}
	ana := read()
	path := "/api/v1/borrowers/" + strconv.FormatInt(ana.ID, 10)
	body := func(stamp, contact string) string {
		return `{"name":"Ana","contact":"` + contact + `","notes":"","updatedAt":"` + stamp + `"}`
	}
	me.expect(http.MethodPut, path, body(ana.UpdatedAt, "ana@example.com"), http.StatusOK)
	stale := decode[struct {
		Errors []struct{ Location string }
	}](t, me.expect(http.MethodPut, path, body(ana.UpdatedAt, "someone else"), http.StatusConflict))
	if len(stale.Errors) != 1 || stale.Errors[0].Location != "body.updatedAt" {
		t.Fatalf("a stale save's errors: %+v", stale)
	}
	me.expect(http.MethodPut, path, body(read().UpdatedAt, "ana@example.org"), http.StatusOK)

	// A share link, saved through sign-in's own package, is refused the same way.
	made := decode[struct {
		Share struct {
			ID        int64  `json:"id"`
			UpdatedAt string `json:"updatedAt"`
		} `json:"share"`
	}](t, me.expect(http.MethodPost, "/api/v1/shares", shareBody(t, "Mom", scope("all", true, "none")), http.StatusCreated))
	link := "/api/v1/shares/" + strconv.FormatInt(made.Share.ID, 10)
	with := func(stamp string) string {
		b := shareBody(t, "Mom and Dad", scope("all", true, "none"))
		return b[:len(b)-1] + `,"updatedAt":"` + stamp + `"}`
	}
	me.expect(http.MethodPut, link, with(made.Share.UpdatedAt), http.StatusOK)
	me.expect(http.MethodPut, link, with(made.Share.UpdatedAt), http.StatusConflict)
}
