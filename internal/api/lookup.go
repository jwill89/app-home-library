package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
	"homelibrary/internal/lookup"
)

type importOutput struct {
	Body *catalog.ImportReport
}

func (h *handlers) registerLookup() {
	huma.Register(h.api, h.op("lookup-isbn", http.MethodGet, "/lookup/isbn/{isbn}", "Ask every source about an ISBN and merge the answers"),
		func(ctx context.Context, in *struct {
			ISBN  string `path:"isbn" maxLength:"20"`
			Fresh bool   `query:"fresh" doc:"Ask the sources again instead of using their last answer"`
		}) (*struct{ Body *catalog.ISBNLookup }, error) {
			res, err := h.Catalog.LookUpISBN(ctx, in.ISBN, in.Fresh)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.ISBNLookup }{Body: res}, nil
		})

	huma.Register(h.api, h.op("lookup-search", http.MethodGet, "/lookup/search", "Find books by title or author words"),
		func(ctx context.Context, in *struct {
			Q string `query:"q" minLength:"2" maxLength:"200"`
		}) (*struct {
			Body struct {
				Candidates []lookup.Candidate `json:"candidates"`
				Failed     map[string]string  `json:"failed"`
			}
		}, error) {
			out := &struct {
				Body struct {
					Candidates []lookup.Candidate `json:"candidates"`
					Failed     map[string]string  `json:"failed"`
				}
			}{}
			out.Body.Candidates, out.Body.Failed = h.Lookup.Search(ctx, strings.TrimSpace(in.Q))
			if out.Body.Candidates == nil {
				out.Body.Candidates = []lookup.Candidate{}
			}
			return out, nil
		})
}

func (h *handlers) registerImports() {
	type exportForm struct {
		File huma.FormFile `form:"file" required:"true" doc:"A CSV or tab-delimited export from Bookshelf, Goodreads, or LibraryThing, or EndNote's XML"`
	}
	imp := h.op("create-import", http.MethodPost, "/imports", "Import another app's export: Bookshelf, Goodreads, or LibraryThing, told by its columns, or EndNote's XML")
	imp.MaxBodyBytes = uploadLimit
	huma.Register(h.api, imp, func(ctx context.Context, in *struct {
		NotOwned string `query:"notOwned" enum:"leave,library,wishlist" default:"leave" doc:"Where the books the other app does not mark as owned go: left out, on the shelves, or on the wishlist"`
		RawBody  huma.MultipartFormFiles[exportForm]
	}) (*importOutput, error) {
		rep, err := h.Catalog.ImportFile(ctx, in.RawBody.Data().File, catalog.ImportOptions{NotOwned: in.NotOwned})
		if err != nil {
			return nil, h.fail(err)
		}
		return &importOutput{Body: rep}, nil
	})

	huma.Register(h.api, h.op("get-enrichment", http.MethodGet, "/enrichment", "How far the background lookup has got"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body *catalog.QueueCounts }, error) {
			c, err := h.Catalog.Counts(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.QueueCounts }{Body: c}, nil
		})
}
