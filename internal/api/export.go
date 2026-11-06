package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

func (h *handlers) registerExport() {
	stamp := func() string { return time.Now().Format("2006-01-02") }

	huma.Register(h.api, h.op("export-library", http.MethodGet, "/export/library.json", "The whole library as one JSON file, to save"),
		func(ctx context.Context, _ *struct{}) (*fileOutput, error) {
			lib, err := h.Catalog.Export(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			b, err := json.MarshalIndent(lib, "", "  ")
			if err != nil {
				return nil, h.fail(err)
			}
			return download("home-library-"+stamp()+".json", "application/json", b), nil
		})

	huma.Register(h.api, h.op("export-books", http.MethodGet, "/export/books.csv", "Every copy as a row of a CSV file, for a spreadsheet"),
		func(ctx context.Context, _ *struct{}) (*fileOutput, error) {
			lib, err := h.Catalog.Export(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			var b bytes.Buffer
			if err := catalog.WriteBooksCSV(&b, lib.Books); err != nil {
				return nil, h.fail(err)
			}
			return download("home-library-books-"+stamp()+".csv", "text/csv; charset=utf-8", b.Bytes()), nil
		})
}
