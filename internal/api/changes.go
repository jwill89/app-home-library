package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2/sse"

	"homelibrary/internal/events"
)

// ping is the stream's heartbeat, an event with nothing in it. A comment line would keep the
// connection open as well, but a browser never shows one to the page, which has to tell a
// quiet library from a line that died without saying so.
type ping struct{}

// reset says the stream's cursor is from another time: everything a page shows is fetched
// again.
type reset struct{}

func (h *handlers) registerChanges() {
	op := h.op("stream-changes", http.MethodGet, "/changes", "The library's changes as they are made")
	op.Description = "Server-Sent Events: each change names a record to fetch again over REST. " +
		"Send Last-Event-ID (the browser does, on its own reconnect) or since to go on from a change."
	sse.Register(h.api, op, map[string]any{"change": events.Change{}, "ping": ping{}, "reset": reset{}},
		func(ctx context.Context, in *struct {
			LastEventID string `header:"Last-Event-ID"`
			Since       int64  `query:"since" minimum:"0" doc:"The last change the page had, when it opens the stream again itself"`
		}, send sse.Sender) {
			since := in.Since
			if id, err := strconv.ParseInt(in.LastEventID, 10, 64); err == nil {
				since = id
			}
			if err := h.Feed.Follow(ctx, since, sender{send}); err != nil {
				h.Logger.Error("change feed", "err", err)
			}
		})
}

// sender is an SSE stream as the feed writes to it.
type sender struct{ send sse.Sender }

func (s sender) Change(c events.Change) error {
	return s.send(sse.Message{IDString: strconv.FormatInt(c.ID, 10), Data: c})
}
func (s sender) Ping() error  { return s.send.Data(ping{}) }
func (s sender) Reset() error { return s.send.Data(reset{}) }
