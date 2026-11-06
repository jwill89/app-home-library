package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

type scanOutput struct {
	Body *catalog.Scan
}

type scansOutput struct {
	Body struct {
		Scans   []catalog.Scan `json:"scans"`
		Pending int64          `json:"pending" doc:"Scans the background is still looking up or adding"`
	}
}

type addScansOutput struct {
	Body struct {
		Adding int64 `json:"adding" doc:"How many of the scans will be added; one already added or let go is not"`
	}
}

type scanID struct {
	ID int64 `path:"id"`
}

func (h *handlers) registerScans() {
	huma.Register(h.api, h.op("identify-code", http.MethodGet, "/codes/{code}", "What a scanned barcode is, and whether the library has its book"),
		func(ctx context.Context, in *struct {
			Code string `path:"code" maxLength:"40"`
		}) (*struct{ Body *catalog.Identified }, error) {
			found, err := h.Catalog.Identify(ctx, in.Code)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Identified }{Body: found}, nil
		})

	queue := h.op("create-scan", http.MethodPost, "/scans", "Queue a scanned ISBN to be looked up and held for the owner")
	queue.DefaultStatus = http.StatusCreated
	huma.Register(h.api, queue, func(ctx context.Context, in *struct{ Body catalog.ScanInput }) (*scanOutput, error) {
		sc, err := h.Catalog.QueueScan(ctx, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &scanOutput{Body: sc}, nil
	})

	huma.Register(h.api, h.op("list-scans", http.MethodGet, "/scans", "The scans not yet marked done"),
		func(ctx context.Context, _ *struct{}) (*scansOutput, error) {
			scans, pending, err := h.Catalog.OpenScans(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &scansOutput{}
			out.Body.Scans, out.Body.Pending = scans, pending
			return out, nil
		})

	huma.Register(h.api, h.op("add-scans", http.MethodPost, "/scans/add", "Add found books to the library, in the background"),
		func(ctx context.Context, in *struct{ Body catalog.AddScansInput }) (*addScansOutput, error) {
			n, err := h.Catalog.AddScans(ctx, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &addScansOutput{}
			out.Body.Adding = n
			return out, nil
		})

	huma.Register(h.api, h.op("finish-scans", http.MethodPost, "/scans/done", "Mark every settled scan done; a found book not added is let go"),
		func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.ReviewFinishedScans(ctx))
		})

	huma.Register(h.api, h.op("finish-scan", http.MethodPost, "/scans/{id}/done", "Mark one scan done; its book stays"),
		func(ctx context.Context, in *scanID) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.ReviewScan(ctx, in.ID))
		})

	huma.Register(h.api, h.op("delete-scan", http.MethodDelete, "/scans/{id}", "Remove a scan made by mistake, and the book it added when asked"),
		func(ctx context.Context, in *struct {
			ID         int64 `path:"id"`
			RemoveBook bool  `query:"removeBook" doc:"Also remove the book this scan added"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.UndoScan(ctx, in.ID, in.RemoveBook))
		})
}

// fail0 is fail for an operation that answers with nothing: nil stays nil.
func (h *handlers) fail0(err error) error {
	if err == nil {
		return nil
	}
	return h.fail(err)
}
