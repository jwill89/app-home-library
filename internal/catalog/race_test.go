package catalog

import (
	"context"
	"net/http"
	"testing"
	"time"

	"homelibrary/internal/lookup"
)

// slowly answers every request after a pause, as a busy source does.
type slowly struct {
	next  http.RoundTripper
	pause time.Duration
}

func (s slowly) RoundTrip(r *http.Request) (*http.Response, error) {
	time.Sleep(s.pause)
	return s.next.RoundTrip(r)
}

func TestLookupKeepsWhatChangedMeanwhile(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	s.Lookup = lookup.New(lookup.Options{UserAgent: "test", Logger: s.Logger,
		HTTPClient: &http.Client{Transport: slowly{s.HTTP.Transport, 300 * time.Millisecond}}})
	id, err := s.Create(ctx, BookInput{Title: "Blood Song", ISBN: "9780425268285", ReadStatus: "unread"}, &CopyInput{PriceCents: -1}, "")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- s.fill(ctx, id, false) }()
	// While the sources are being asked, the owner marks the book read.
	time.Sleep(100 * time.Millisecond)
	if err := s.SetReadStatus(ctx, id, ReadStatusInput{ReadStatus: "read", On: "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	b, _ := s.Get(ctx, id)
	if b.ReadStatus != "read" {
		t.Errorf("the lookup undid the owner's change: %q", b.ReadStatus)
	}
	if b.Publisher == "" || b.Series == "" {
		t.Errorf("the lookup's details were not merged: publisher %q, series %q", b.Publisher, b.Series)
	}
}
