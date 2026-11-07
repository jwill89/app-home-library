package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

type gapsOutput struct {
	Body struct {
		Books []catalog.BookGaps `json:"books"`
	}
}

// changedIDs is a list's ?ids=: only the books the change feed named, each asked about again.
type changedIDs struct {
	IDs []int64 `query:"ids" maxItems:"500" doc:"Only these books, as the change feed names them; one not among the answer is no longer listed"`
}

type queuedOutput struct {
	Body struct {
		Queued int64 `json:"queued" doc:"Books queued; one without an ISBN is left out"`
	}
}

func (h *handlers) registerGaps() {
	huma.Register(h.api, h.op("list-gaps", http.MethodGet, "/gaps", "The books that lack a detail, with what each lacks"),
		func(ctx context.Context, in *changedIDs) (*gapsOutput, error) {
			books, err := h.Catalog.Gaps(ctx, in.IDs)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &gapsOutput{}
			out.Body.Books = books
			return out, nil
		})

	huma.Register(h.api, h.op("queue-lookups", http.MethodPost, "/enrichment", "Put books in the background lookup's queue, to fill what they lack, each source asked again"),
		func(ctx context.Context, in *struct {
			Body struct {
				IDs []int64 `json:"ids,omitempty" maxItems:"5000" doc:"The books to look up again"`
				All bool    `json:"all,omitempty" doc:"Every book with an ISBN, in place of ids"`
			}
		}) (*queuedOutput, error) {
			var n int64
			var err error
			switch {
			case in.Body.All:
				n, err = h.Catalog.QueueEveryLookup(ctx)
			case len(in.Body.IDs) > 0:
				n, err = h.Catalog.QueueLookups(ctx, in.Body.IDs)
			default:
				return nil, huma.Error422UnprocessableEntity("Name the books to look up again, or ask for all of them.")
			}
			if err != nil {
				return nil, h.fail(err)
			}
			out := &queuedOutput{}
			out.Body.Queued = n
			return out, nil
		})
}
