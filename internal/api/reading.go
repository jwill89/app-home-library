package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
)

func (h *handlers) registerReading() {
	huma.Register(h.api, h.readOp("get-reading", "/reading", "Every read finished, and the books and pages each year"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body *catalog.ReadingStats }, error) {
			p, _ := auth.FromContext(ctx)
			stats, err := h.Catalog.Reading(ctx, p.Scope)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.ReadingStats }{Body: stats}, nil
		})

	again := h.op("read-again", http.MethodPost, "/books/{id}/read-again", "Begin a book again; the read it had is kept among its earlier reads")
	huma.Register(h.api, again, func(ctx context.Context, in *struct {
		ID   int64 `path:"id"`
		Body struct {
			On string `json:"on" doc:"YYYY-MM-DD, on the owner's device"`
		}
	}) (*bookOutput, error) {
		if err := h.Catalog.ReadAgain(ctx, in.ID, in.Body.On); err != nil {
			return nil, h.fail(err)
		}
		return h.book(ctx, in.ID)
	})

	huma.Register(h.api, h.op("set-read-status", http.MethodPut, "/books/{id}/read-status", "Change a book's reading status as it happens"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.ReadStatusInput
		}) (*bookOutput, error) {
			if err := h.Catalog.SetReadStatus(ctx, in.ID, in.Body); err != nil {
				return nil, h.fail(err)
			}
			return h.book(ctx, in.ID)
		})

	huma.Register(h.api, h.op("set-series-read", http.MethodPut, "/series/{id}/read-status", "Mark every book of a series the house has read, or unread"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.SeriesReadInput
		}) (*struct {
			Body struct {
				Changed int64 `json:"changed" doc:"How many books changed"`
			}
		}, error) {
			n, err := h.Catalog.SetSeriesRead(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Changed int64 `json:"changed" doc:"How many books changed"`
				}
			}{}
			out.Body.Changed = n
			return out, nil
		})

	huma.Register(h.api, h.op("delete-reading", http.MethodDelete, "/readings/{id}", "Remove an earlier read recorded by mistake"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.DeleteReading(ctx, in.ID))
		})
}
