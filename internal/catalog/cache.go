package catalog

import (
	"context"
	"log/slog"
	"time"

	"homelibrary/internal/db"
)

// LookupCache keeps the sources' raw answers in the lookups table.
type LookupCache struct {
	DB     *db.DB
	Logger *slog.Logger
}

// Get returns a cached answer and when it was fetched.
func (c *LookupCache) Get(ctx context.Context, key, source string) ([]byte, time.Time, bool) {
	row, err := db.New(c.DB.Read).GetLookup(ctx, db.GetLookupParams{LookupKey: key, Source: source})
	if err != nil {
		return nil, time.Time{}, false
	}
	at, err := time.Parse("2006-01-02T15:04:05Z", row.FetchedAt)
	if err != nil {
		return nil, time.Time{}, false
	}
	return row.Body, at, true
}

// Put stores an answer. A failure costs a second request later, so it is logged and no more.
func (c *LookupCache) Put(ctx context.Context, key, source string, body []byte) {
	if body == nil {
		body = []byte{}
	}
	if err := db.New(c.DB.Write).PutLookup(ctx, db.PutLookupParams{LookupKey: key, Source: source, Body: body}); err != nil {
		c.Logger.Warn("lookup cache", "source", source, "err", err)
	}
}
