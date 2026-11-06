package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
)

func (h *handlers) registerSeries() {
	huma.Register(h.api, h.readOp("list-series", "/series", "Every series with the volumes had and wanted, the gaps, and the next number"),
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Series []catalog.Series `json:"series"`
			}
		}, error) {
			p, _ := auth.FromContext(ctx)
			list, err := h.Catalog.SeriesList(ctx, p.Scope)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Series []catalog.Series `json:"series"`
				}
			}{}
			out.Body.Series = list
			return out, nil
		})

	huma.Register(h.api, h.readOp("get-series", "/series/{id}", "One series with the volumes had and wanted, the gaps, and the next number"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body *catalog.Series }, error) {
			p, _ := auth.FromContext(ctx)
			se, err := h.Catalog.GetSeries(ctx, in.ID, p.Scope)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Series }{Body: se}, nil
		})

	huma.Register(h.api, h.op("leave-series", http.MethodDelete, "/series/{id}/books/{bookId}", "Take a book out of a series; no lookup puts it in one again"),
		func(ctx context.Context, in *struct {
			ID     int64 `path:"id"`
			BookID int64 `path:"bookId"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.LeaveSeries(ctx, in.ID, in.BookID))
		})

	huma.Register(h.api, h.op("dismiss-series", http.MethodDelete, "/series/{id}", "Say a series is none: its books leave it, and no lookup or import puts a book in it again"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct {
			Body struct {
				Changed int64 `json:"changed" doc:"How many books left it"`
			}
		}, error) {
			n, err := h.Catalog.DismissSeries(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Changed int64 `json:"changed" doc:"How many books left it"`
				}
			}{}
			out.Body.Changed = n
			return out, nil
		})

	huma.Register(h.api, h.op("rename-series", http.MethodPut, "/series/{id}", "Rename a series; another series' name merges the two"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.SeriesInput
		}) (*struct {
			Body struct {
				ID int64 `json:"id" doc:"The series that remains: this one, or the one it merged into"`
			}
		}, error) {
			keep, err := h.Catalog.RenameSeries(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					ID int64 `json:"id" doc:"The series that remains: this one, or the one it merged into"`
				}
			}{}
			out.Body.ID = keep
			return out, nil
		})
}
