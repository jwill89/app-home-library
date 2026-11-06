package api

import (
	"context"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
)

type bookOutput struct {
	Body *catalog.Book
}

type bookID struct {
	ID int64 `path:"id"`
}

type coverForm struct {
	File huma.FormFile `form:"file" contentType:"image/jpeg,image/png,image/webp" required:"true" doc:"The cover, photographed or uploaded"`
}

// Uploads are the one place a request is large: a phone photo before the browser shrinks it.
const uploadLimit = 16 << 20

func (h *handlers) book(ctx context.Context, id int64) (*bookOutput, error) {
	b, err := h.Catalog.Get(ctx, id)
	if err != nil {
		return nil, h.fail(err)
	}
	return &bookOutput{Body: b}, nil
}

func (h *handlers) registerBooks() {
	huma.Register(h.api, h.readOp("list-books", "/books", "Every book in the library, or those asked for"),
		func(ctx context.Context, in *struct {
			IDs []int64 `query:"ids" maxItems:"500" doc:"Only these books, as the change feed names them; one not among the answer is gone"`
		}) (*struct {
			Body struct {
				Books []catalog.BookRow `json:"books"`
			}
		}, error) {
			rows, err := h.Catalog.List(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			if len(in.IDs) > 0 {
				rows = slices.DeleteFunc(rows, func(r catalog.BookRow) bool { return !slices.Contains(in.IDs, r.ID) })
			}
			p, _ := auth.FromContext(ctx)
			rows = p.Scope.Rows(rows)
			out := &struct {
				Body struct {
					Books []catalog.BookRow `json:"books"`
				}
			}{}
			out.Body.Books = rows
			return out, nil
		})

	create := h.op("create-book", http.MethodPost, "/books", "Add a book with its first copy, or to the wishlist without one")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, in *struct {
		Body struct {
			Book     catalog.BookInput `json:"book"`
			Copy     catalog.CopyInput `json:"copy"`
			CoverURL string            `json:"coverUrl" maxLength:"2000" doc:"A cover candidate from the lookup to fetch; empty for none"`
			Wanted   bool              `json:"wanted" doc:"Put it on the wishlist instead: no copy is made"`
		}
	}) (*bookOutput, error) {
		first := &in.Body.Copy
		if in.Body.Wanted {
			first = nil
		}
		id, err := h.Catalog.Create(ctx, in.Body.Book, first, in.Body.CoverURL)
		if err != nil {
			return nil, h.fail(err)
		}
		return h.book(ctx, id)
	})

	huma.Register(h.api, h.readOp("get-book", "/books/{id}", "A book with its copies; a visitor's leaves out what is the owner's alone"),
		func(ctx context.Context, in *bookID) (*bookOutput, error) {
			out, err := h.book(ctx, in.ID)
			if p, _ := auth.FromContext(ctx); err == nil && !p.Owner() {
				if !p.Scope.ShowsBook(out.Body) {
					return nil, h.fail(catalog.ErrNotFound)
				}
				out.Body = out.Body.ForVisitor()
			}
			return out, err
		})

	huma.Register(h.api, h.op("update-book", http.MethodPut, "/books/{id}", "Replace a book's details"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.BookInput
		}) (*bookOutput, error) {
			if err := h.Catalog.Update(ctx, in.ID, in.Body); err != nil {
				return nil, h.fail(err)
			}
			return h.book(ctx, in.ID)
		})

	huma.Register(h.api, h.op("delete-book", http.MethodDelete, "/books/{id}", "Remove a book and its copies"),
		func(ctx context.Context, in *bookID) (*struct{}, error) {
			if err := h.Catalog.Delete(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})

	huma.Register(h.api, h.op("refresh-book", http.MethodPost, "/books/{id}/refresh", "Look the book up again: fill its blanks, and suggest what the sources now give otherwise"),
		func(ctx context.Context, in *bookID) (*struct{ Body *catalog.Suggestions }, error) {
			s, err := h.Catalog.Refresh(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Suggestions }{Body: s}, nil
		})

	upload := h.op("upload-cover", http.MethodPut, "/books/{id}/cover", "Make a photo or an image file the cover")
	upload.MaxBodyBytes = uploadLimit
	huma.Register(h.api, upload, func(ctx context.Context, in *struct {
		ID      int64 `path:"id"`
		RawBody huma.MultipartFormFiles[coverForm]
	}) (*bookOutput, error) {
		f := in.RawBody.Data().File
		if err := h.Catalog.CoverFromUpload(ctx, in.ID, f); err != nil {
			return nil, h.fail(err)
		}
		return h.book(ctx, in.ID)
	})

	huma.Register(h.api, h.op("fetch-cover", http.MethodPost, "/books/{id}/cover", "Fetch a cover from a link"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body struct {
				URL string `json:"url" minLength:"8" maxLength:"2000" pattern:"^https?://"`
			}
		}) (*bookOutput, error) {
			if err := h.Catalog.CoverFromURL(ctx, in.ID, in.Body.URL); err != nil {
				return nil, huma.Error422UnprocessableEntity("That cover could not be fetched: " + err.Error())
			}
			return h.book(ctx, in.ID)
		})

	huma.Register(h.api, h.op("delete-cover", http.MethodDelete, "/books/{id}/cover", "Leave the book without a cover"),
		func(ctx context.Context, in *bookID) (*bookOutput, error) {
			if err := h.Catalog.RemoveCover(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return h.book(ctx, in.ID)
		})

	huma.Register(h.api, h.op("get-choices", http.MethodGet, "/choices", "Sections, genres, and series already in use"),
		func(ctx context.Context, _ *struct{}) (*struct{ Body *catalog.Choices }, error) {
			c, err := h.Catalog.Choices(ctx)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.Choices }{Body: c}, nil
		})
}

func (h *handlers) registerCopies() {
	type copyOutput struct {
		Body *catalog.Copy
	}
	add := h.op("create-copy", http.MethodPost, "/books/{id}/copies", "Record another copy of a book")
	add.DefaultStatus = http.StatusCreated
	huma.Register(h.api, add, func(ctx context.Context, in *struct {
		ID   int64 `path:"id"`
		Body catalog.CopyInput
	}) (*copyOutput, error) {
		c, err := h.Catalog.AddCopy(ctx, in.ID, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &copyOutput{Body: c}, nil
	})

	huma.Register(h.api, h.op("update-copy", http.MethodPut, "/copies/{id}", "Replace a copy's details"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.CopyInput
		}) (*copyOutput, error) {
			c, err := h.Catalog.UpdateCopy(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &copyOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("mark-copy-gone", http.MethodPut, "/copies/{id}/gone", "Record that a copy has left the library: lost, sold, donated, given away, or discarded"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.GoneInput
		}) (*copyOutput, error) {
			c, err := h.Catalog.MarkGone(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &copyOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("bring-copy-back", http.MethodDelete, "/copies/{id}/gone", "Put a copy that had gone back in the library"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*copyOutput, error) {
			c, err := h.Catalog.BringBack(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &copyOutput{Body: c}, nil
		})

	huma.Register(h.api, h.op("mark-copy-seen", http.MethodPut, "/copies/{id}/seen", "Record the day a shelf check found a copy"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body struct {
				On string `json:"on" doc:"YYYY-MM-DD, on the owner's device"`
			}
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.MarkSeen(ctx, in.ID, in.Body.On))
		})

	huma.Register(h.api, h.op("delete-copy", http.MethodDelete, "/copies/{id}", "Remove one copy recorded by mistake; the book stays"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Catalog.DeleteCopy(ctx, in.ID); err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})
}
