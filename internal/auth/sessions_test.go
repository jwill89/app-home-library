package auth

import (
	"bytes"
	"context"
	"testing"
	"time"

	"homelibrary/internal/db"
	"homelibrary/internal/db/dbtest"
)

// A session is found while it runs and not after, written to the second in the timestamp
// form, replaced when it changes, and let go once it ends.
func TestSessionStore(t *testing.T) {
	d := dbtest.Open(t)
	ctx := context.Background()
	s := sessionStore{read: d.Read, write: d.Write}

	if err := s.Commit("running", []byte("one"), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit("running", []byte("two"), time.Now().Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if b, ok, err := s.Find("running"); err != nil || !ok || !bytes.Equal(b, []byte("two")) {
		t.Fatalf("a running session: %q %v %v", b, ok, err)
	}
	var expiry string
	if err := d.Read.QueryRowContext(ctx, "SELECT expiry FROM sessions WHERE token = 'running'").Scan(&expiry); err != nil {
		t.Fatal(err)
	}
	if _, err := time.Parse(time.RFC3339, expiry); err != nil || len(expiry) != len("2026-11-07T09:18:09Z") {
		t.Errorf("expiry kept as %q", expiry)
	}

	if err := s.Commit("ended", []byte("old"), time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Find("ended"); err != nil || ok {
		t.Errorf("an ended session was found: %v %v", ok, err)
	}
	if _, ok, err := s.Find("never"); err != nil || ok {
		t.Errorf("a session never made was found: %v %v", ok, err)
	}
	if n, err := db.New(d.Write).DeleteExpiredSessions(ctx); err != nil || n != 1 {
		t.Errorf("pruned %d, err %v", n, err)
	}

	if err := s.Delete("running"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := s.Find("running"); ok {
		t.Error("a deleted session was found")
	}
}

// The pruner stops when its context does.
func TestSessionPrunerStops(t *testing.T) {
	d := dbtest.Open(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunSessionPruner(ctx, d.Write, nil)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the pruner kept running")
	}
}
