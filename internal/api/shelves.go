package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

// The shelves are the owner's: where a book stands in the house is never a visitor's to see.
func (h *handlers) registerShelves() {
	type bookcases struct {
		Body struct {
			Bookcases []catalog.Bookcase `json:"bookcases"`
		}
	}
	list := func(ctx context.Context) (*bookcases, error) {
		all, err := h.Catalog.Bookcases(ctx)
		if err != nil {
			return nil, h.fail(err)
		}
		out := &bookcases{}
		out.Body.Bookcases = all
		return out, nil
	}

	huma.Register(h.api, h.op("list-bookcases", http.MethodGet, "/bookcases", "The bookcases in walking order, with their shelves"),
		func(ctx context.Context, _ *struct{}) (*bookcases, error) { return list(ctx) })

	create := h.op("create-bookcase", http.MethodPost, "/bookcases", "Add a bookcase with its shelves, all alike to begin with")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, in *struct{ Body catalog.NewBookcase }) (*bookcases, error) {
		if _, err := h.Catalog.CreateBookcase(ctx, in.Body); err != nil {
			return nil, h.fail(err)
		}
		return list(ctx)
	})

	huma.Register(h.api, h.op("update-bookcase", http.MethodPut, "/bookcases/{id}", "Rename a bookcase or move it in the walking order"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.BookcaseInput
		}) (*bookcases, error) {
			if err := h.Catalog.UpdateBookcase(ctx, in.ID, in.Body); err != nil {
				return nil, h.fail(err)
			}
			return list(ctx)
		})

	huma.Register(h.api, h.op("delete-bookcase", http.MethodDelete, "/bookcases/{id}", "Remove a bookcase and its shelves; the books are only planned again"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.DeleteBookcase(ctx, in.ID))
		})

	addShelf := h.op("create-shelf", http.MethodPost, "/bookcases/{id}/shelves", "Add a shelf at the bottom of a bookcase")
	addShelf.DefaultStatus = http.StatusCreated
	huma.Register(h.api, addShelf, func(ctx context.Context, in *struct {
		ID   int64 `path:"id"`
		Body catalog.ShelfInput
	}) (*bookcases, error) {
		if err := h.Catalog.AddShelf(ctx, in.ID, in.Body); err != nil {
			return nil, h.fail(err)
		}
		return list(ctx)
	})

	huma.Register(h.api, h.op("update-shelf", http.MethodPut, "/shelves/{id}", "Change a shelf's measurements, or whether it takes oversize books"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.ShelfInput
		}) (*bookcases, error) {
			if err := h.Catalog.UpdateShelf(ctx, in.ID, in.Body); err != nil {
				return nil, h.fail(err)
			}
			return list(ctx)
		})

	huma.Register(h.api, h.op("delete-shelf", http.MethodDelete, "/shelves/{id}", "Remove a shelf; the shelves below move up a number"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.DeleteShelf(ctx, in.ID))
		})

	huma.Register(h.api, h.op("get-layout", http.MethodGet, "/layout", "The library on its shelves: the layout in use, or a proposal"),
		func(ctx context.Context, in *struct {
			Proposal bool `query:"proposal" doc:"A fresh proposal, even while a layout is in use"`
		}) (*struct{ Body *catalog.Layout }, error) {
			l, err := h.Catalog.Layout(ctx, in.Proposal)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Layout }{Body: l}, nil
		})

	huma.Register(h.api, h.op("set-layout", http.MethodPut, "/layout", "Set how full the planner fills a shelf"),
		func(ctx context.Context, in *struct {
			Body struct {
				FillPercent int64 `json:"fillPercent" minimum:"50" maximum:"100"`
			}
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.SetFillPercent(ctx, in.Body.FillPercent))
		})

	huma.Register(h.api, h.op("use-layout", http.MethodPut, "/layout/in-use", "Put the proposal to use: each shelf keeps where it starts in the order"),
		func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.UseLayout(ctx))
		})

	huma.Register(h.api, h.op("stop-layout", http.MethodDelete, "/layout/in-use", "Stop using the layout: the planner proposes afresh"),
		func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.StopLayout(ctx))
		})

	huma.Register(h.api, h.op("shelf-labels", http.MethodGet, "/layout/labels", "A label for the edge of every shelf with books, as a PDF on the spine-label sheet"),
		func(ctx context.Context, _ *struct{}) (*fileOutput, error) {
			b, err := h.Catalog.RangeLabelsPDF(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			return pdf("shelf-labels", b), nil
		})

	huma.Register(h.api, h.op("get-copy-place", http.MethodGet, "/copies/{id}/place", "Where a copy stands on the shelves"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body *catalog.ShelfPlace }, error) {
			p, err := h.Catalog.PlaceOf(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.ShelfPlace }{Body: p}, nil
		})
}
