package events

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"homelibrary/internal/db"
)

// sink is a stream as a test reads it.
type sink struct {
	changes chan Change
	resets  chan struct{}
}

func newSink() *sink {
	return &sink{changes: make(chan Change, 100), resets: make(chan struct{}, 10)}
}

func (s *sink) Change(c Change) error { s.changes <- c; return nil }
func (s *sink) Ping() error           { return nil }
func (s *sink) Reset() error          { s.resets <- struct{}{}; return nil }

func (s *sink) next(t *testing.T) Change {
	t.Helper()
	select {
	case c := <-s.changes:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("no change came")
		return Change{}
	}
}

func (s *sink) quiet(t *testing.T) {
	t.Helper()
	select {
	case c := <-s.changes:
		t.Fatalf("a change came: %+v", c)
	case <-time.After(150 * time.Millisecond):
	}
}

func newFeed(t *testing.T) (*Feed, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "feed.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := db.Migrate(context.Background(), database.Write); err != nil {
		t.Fatal(err)
	}
	f := New(database.Read, database.Write, slog.New(slog.NewTextHandler(io.Discard, nil)))
	f.Poll = 10 * time.Millisecond
	f.Lookback = 0
	return f, database
}

func exec(t *testing.T, d *db.DB, query string) {
	t.Helper()
	if _, err := d.Write.ExecContext(context.Background(), query); err != nil {
		t.Fatal(err)
	}
}

func follow(t *testing.T, f *Feed, since int64) *sink {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	out := newSink()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := f.Follow(ctx, since, out); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	time.Sleep(30 * time.Millisecond) // the stream has its starting cursor
	return out
}

func TestFollow(t *testing.T) {
	f, d := newFeed(t)
	exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'Before')`)

	// A new stream starts from now: what was made before it is the page's first fetch.
	live := follow(t, f, 0)
	live.quiet(t)
	exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'After')`)
	if c := live.next(t); c.Type != "document" || c.Ref != 2 {
		t.Fatalf("change %+v", c)
	}
	// A record changed three times between looks goes once, as its last change.
	exec(t, d, `UPDATE documents SET title = title || '.' WHERE id = 1;
		UPDATE documents SET title = title || '.' WHERE id = 1;
		UPDATE documents SET title = title || '.' WHERE id = 1`)
	last := live.next(t)
	if last.Ref != 1 {
		t.Fatalf("change %+v", last)
	}
	live.quiet(t)

	// A stream that dropped goes on from the last change it had, missing nothing.
	exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'While away')`)
	again := follow(t, f, last.ID)
	if c := again.next(t); c.Ref != 3 {
		t.Fatalf("resumed with %+v", c)
	}

	// A cursor ahead of the feed, as after a backup is put back, is told to fetch everything.
	ahead := follow(t, f, last.ID+1000)
	select {
	case <-ahead.resets:
	case <-time.After(time.Second):
		t.Fatal("a cursor from another time was not reset")
	}
}

// A new stream starts a moment back: a change made after the page fetched, but before its
// stream opened, still reaches it.
func TestFollowLooksBack(t *testing.T) {
	f, d := newFeed(t)
	exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'Long ago')`)
	exec(t, d, `UPDATE changes SET at = '2000-01-01T00:00:00.000Z'`)
	exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'Just now')`)
	f.Lookback = time.Minute
	late := follow(t, f, 0)
	if c := late.next(t); c.Ref != 2 {
		t.Fatalf("change %+v", c)
	}
	late.quiet(t)
}

// One past the newest kept is told to fetch everything; the stream goes on from now.
func TestFollowPrunedCursor(t *testing.T) {
	f, d := newFeed(t)
	for range 5 {
		exec(t, d, `INSERT INTO documents (kind, title) VALUES ('other', 'A')`)
	}
	if _, err := f.write.PruneChanges(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	old := follow(t, f, 1)
	select {
	case <-old.resets:
	case <-time.After(time.Second):
		t.Fatal("a pruned cursor was not reset")
	}
	old.quiet(t)
}

// Closing the feed ends its streams, so the server's shutdown does not wait them out.
func TestClose(t *testing.T) {
	f, _ := newFeed(t)
	done := make(chan error)
	go func() { done <- f.Follow(context.Background(), 0, newSink()) }()
	f.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("the stream outlived the feed")
	}
}
