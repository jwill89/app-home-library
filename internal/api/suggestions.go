package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
)

type suggestionListOutput struct {
	Body struct {
		Books []catalog.SuggestedBook `json:"books"`
	}
}

func (h *handlers) registerSuggestions() {
	huma.Register(h.api, h.op("list-suggestions", http.MethodGet, "/suggestions", "The books whose last lookup the owner asked for gives details otherwise, with which details"),
		func(ctx context.Context, in *changedIDs) (*suggestionListOutput, error) {
			books, err := h.Catalog.SuggestionList(ctx, in.IDs)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &suggestionListOutput{}
			out.Body.Books = books
			return out, nil
		})

	huma.Register(h.api, h.op("get-suggestions", http.MethodGet, "/books/{id}/suggestions", "What the book's last lookup the owner asked for offers to change now"),
		func(ctx context.Context, in *bookID) (*struct{ Body *catalog.Suggestions }, error) {
			s, err := h.Catalog.Suggested(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Suggestions }{Body: s}, nil
		})

	huma.Register(h.api, h.op("decline-suggestions", http.MethodPost, "/books/{id}/declined-suggestions", "Keep the book as it is in these details: what the sources give for them is not offered again while they give it"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.Declined
		}) (*struct{}, error) {
			if err := h.Catalog.DeclineSuggestions(ctx, in.ID, in.Body.Fields); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})
}
