// Package events is the change feed. Triggers write a row to the changes table in the
// transaction that makes each change, whichever process makes it: the web server, the text
// service, or an import run by hand. The feed reads that table for each stream open on it and
// sends the changes on; a page fetches again what it shows. Being a table, the feed outlives
// a restart, and a stream that drops goes on from the last change it had.
package events

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"homelibrary/internal/db"
)

// Change is one record that changed. What it is now is fetched over REST; one no longer
// there was removed.
type Change struct {
	ID   int64  `json:"id" doc:"The change's place in the feed, sent back as Last-Event-ID (or since) to go on from it"`
	Type string `json:"type" enum:"book,scan,document,lookup,loan,label" doc:"What changed: a book (one edition), a scan, a paper, a book's background lookup, a loan, or a copy's label"`
	Ref  int64  `json:"ref" doc:"Its id: the book's for a book and its lookup, the copy's for a label"`
}

// Sink is a stream: where the feed sends its changes, a heartbeat, and word that the stream's
// cursor is from another time, so everything a page shows is to be fetched again.
type Sink interface {
	Change(c Change) error
	Ping() error
	Reset() error
}

// Feed reads the changes for the streams open on it.
type Feed struct {
	read, write *db.Queries
	logger      *slog.Logger
	// How often a stream looks for changes, and how often it says it is alive. A change made in
	// this process or another reaches a page within Poll; Cloudflare closes a connection quiet
	// for 100 seconds, so a ping comes well inside that.
	Poll, Ping time.Duration
	// How far back a new stream starts. A page fetches what it shows, then opens its stream: a
	// change made between the two would otherwise be missed. It sends a few changes the page
	// had already, which it fetches again for nothing.
	Lookback time.Duration
	done     chan struct{}
	closing  sync.Once
}

// keep is how many of the newest changes are kept: a page away longer than that many changes
// is told to fetch everything again.
const keep = 10_000

// New makes the feed over the database's read pool, and its write connection for pruning.
func New(read, write *sql.DB, logger *slog.Logger) *Feed {
	return &Feed{read: db.New(read), write: db.New(write), logger: logger,
		Poll: 500 * time.Millisecond, Ping: 25 * time.Second, Lookback: 5 * time.Second, done: make(chan struct{})}
}

// Close ends every stream, as the server shuts down: a stream is a request that would
// otherwise hold the shutdown open until it timed out.
func (f *Feed) Close() {
	f.closing.Do(func() { close(f.done) })
}

// Follow sends a stream the changes after its cursor as they are made, until the request ends
// or the feed closes. A cursor of 0 is a new stream, which starts Lookback ago. One from another
// time (pruned past, or ahead of a database put back from a backup) is told to fetch
// everything, and goes on from now.
func (f *Feed) Follow(ctx context.Context, since int64, out Sink) error {
	cursor, err := f.start(ctx, since, out)
	if err != nil {
		return err
	}
	poll := time.NewTicker(f.Poll)
	defer poll.Stop()
	ping := time.NewTicker(f.Ping)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-f.done:
			return nil
		case <-ping.C:
			if out.Ping() != nil {
				return nil // the page has gone
			}
		case <-poll.C:
			if cursor, err = f.send(ctx, cursor, out); errors.Is(err, errGone) || ctx.Err() != nil {
				return nil
			} else if err != nil {
				return err
			}
		}
	}
}

// errGone is a stream whose page has gone: the request ends with it, and nothing is wrong.
var errGone = errors.New("the page has gone")

func (f *Feed) start(ctx context.Context, since int64, out Sink) (int64, error) {
	if since <= 0 {
		from, err := f.read.ChangeBefore(ctx, time.Now().UTC().Add(-f.Lookback).Format(changeTime))
		if err != nil {
			return 0, fmt.Errorf("feed start: %w", err)
		}
		return from, nil
	}
	last, err := f.read.LastChange(ctx)
	if err != nil {
		return 0, fmt.Errorf("feed start: %w", err)
	}
	first, err := f.read.FirstChange(ctx)
	if err != nil {
		return 0, fmt.Errorf("feed start: %w", err)
	}
	if since > last || since < first-1 {
		if err := out.Reset(); err != nil {
			return 0, err
		}
		return last, nil
	}
	return since, nil
}

// send hands on every change after the cursor and returns the new cursor. A record changed
// several times since the last look goes once, as its last change: the page fetches it once.
func (f *Feed) send(ctx context.Context, cursor int64, out Sink) (int64, error) {
	for {
		rows, err := f.read.ListChanges(ctx, cursor)
		if err != nil {
			return cursor, fmt.Errorf("feed: %w", err)
		}
		if len(rows) == 0 {
			return cursor, nil
		}
		type record struct {
			kind string
			ref  int64
		}
		latest := map[record]int{}
		for i, r := range rows {
			latest[record{r.Type, r.Ref}] = i
		}
		for i, r := range rows {
			if latest[record{r.Type, r.Ref}] != i {
				continue
			}
			if out.Change(Change{ID: r.ID, Type: r.Type, Ref: r.Ref}) != nil {
				return cursor, errGone
			}
		}
		cursor = rows[len(rows)-1].ID
		if len(rows) < changesPage {
			return cursor, nil
		}
	}
}

// changeTime is how the changes table writes its times: strftime's %Y-%m-%dT%H:%M:%fZ.
const changeTime = "2006-01-02T15:04:05.000Z"

// changesPage is ListChanges' LIMIT: a full page means more may wait.
const changesPage = 1000

// RunPruner lets the oldest changes go, once an hour, keeping the newest.
func (f *Feed) RunPruner(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := f.write.PruneChanges(ctx, keep); err != nil && ctx.Err() == nil {
				f.logger.Error("prune changes", "err", err)
			}
		}
	}
}
