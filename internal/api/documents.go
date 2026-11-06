package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/lookup"
)

type documentOutput struct {
	Body *catalog.Document
}

type openCopyBody struct {
	Found bool            `json:"found" doc:"Whether OpenAlex knows an open copy; false for a document without a DOI"`
	Copy  lookup.OpenCopy `json:"copy"`
}

type fileForm struct {
	File huma.FormFile `form:"file" required:"true" doc:"A PDF, or a JPEG, PNG, or WebP photo of a page; known by its first bytes, whatever type the browser gives"`
}

// Documents are the owner's; a visitor reads only those their link shows, without the notes,
// the course, or the text read from them.
func (h *handlers) registerDocuments() {
	huma.Register(h.api, h.readOp("list-documents", "/documents", "Every document, or those whose details or files have the words searched for"),
		func(ctx context.Context, in *struct {
			Q string `query:"q" maxLength:"200" doc:"Words to search for in titles, authors, notes, and the files' text"`
		}) (*struct {
			Body struct {
				Documents []catalog.DocumentRow `json:"documents"`
			}
		}, error) {
			p, _ := auth.FromContext(ctx)
			rows, err := h.Catalog.Documents(ctx, in.Q, p.Scope)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Documents []catalog.DocumentRow `json:"documents"`
				}
			}{}
			out.Body.Documents = rows
			return out, nil
		})

	create := h.op("create-document", http.MethodPost, "/documents", "Record a document; its files come after")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, in *struct{ Body catalog.DocumentInput }) (*documentOutput, error) {
		d, err := h.Catalog.CreateDocument(ctx, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &documentOutput{Body: d}, nil
	})

	huma.Register(h.api, h.readOp("get-document", "/documents/{id}", "A document with its files"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*documentOutput, error) {
			d, err := h.Catalog.GetDocument(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			if p, _ := auth.FromContext(ctx); !p.Owner() {
				if !p.Scope.ShowsPaper(d.ID, d.CourseID) {
					return nil, h.fail(catalog.ErrNotFound)
				}
				d = d.ForVisitor()
			}
			return &documentOutput{Body: d}, nil
		})

	huma.Register(h.api, h.op("update-document", http.MethodPut, "/documents/{id}", "Replace a document's details; its files stay"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body catalog.DocumentInput
		}) (*documentOutput, error) {
			d, err := h.Catalog.UpdateDocument(ctx, in.ID, in.Body)
			if err != nil {
				return nil, h.fail(err)
			}
			return &documentOutput{Body: d}, nil
		})

	huma.Register(h.api, h.op("delete-document", http.MethodDelete, "/documents/{id}", "Remove a document and its files"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			return nil, h.fail0(h.Catalog.DeleteDocument(ctx, in.ID))
		})

	upload := h.op("add-document-file", http.MethodPost, "/documents/{id}/files", "Add a PDF or a photo of a page to a document")
	upload.MaxBodyBytes = catalog.FileLimit + 1<<20
	huma.Register(h.api, upload, func(ctx context.Context, in *struct {
		ID      int64 `path:"id"`
		RawBody huma.MultipartFormFiles[fileForm]
	}) (*documentOutput, error) {
		f := in.RawBody.Data().File
		d, err := h.Catalog.AddDocumentFile(ctx, in.ID, f.Filename, f)
		if err != nil {
			return nil, h.fail(err)
		}
		return &documentOutput{Body: d}, nil
	})

	huma.Register(h.api, h.readOp("get-document-file", "/documents/{id}/files/{fileId}", "One of a document's files, to view or download"),
		func(ctx context.Context, in *struct {
			ID     int64 `path:"id"`
			FileID int64 `path:"fileId"`
		}) (*huma.StreamResponse, error) {
			p, _ := auth.FromContext(ctx)
			path, name, contentType, err := h.Catalog.FileForReading(ctx, in.ID, in.FileID, p.Scope)
			if err != nil {
				return nil, h.fail(err)
			}
			return &huma.StreamResponse{Body: func(hctx huma.Context) {
				r, w := humago.Unwrap(hctx)
				f, err := os.Open(path) //nolint:gosec // a path the store built from a stored name it checked
				if err != nil {
					h.Logger.Error("document file", "path", path, "err", err)
					http.Error(w, "the file could not be read", http.StatusInternalServerError)
					return
				}
				defer func() { _ = f.Close() }()
				info, err := f.Stat()
				if err != nil {
					http.Error(w, "the file could not be read", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", contentType)
				w.Header().Set("Content-Disposition", "inline; filename*=UTF-8''"+url.PathEscape(name))
				// ServeContent answers the range requests a PDF viewer makes.
				http.ServeContent(w, r, "", info.ModTime(), f)
			}}, nil
		})

	huma.Register(h.api, h.op("delete-document-file", http.MethodDelete, "/documents/{id}/files/{fileId}", "Remove one of a document's files"),
		func(ctx context.Context, in *struct {
			ID     int64 `path:"id"`
			FileID int64 `path:"fileId"`
		}) (*documentOutput, error) {
			d, err := h.Catalog.RemoveDocumentFile(ctx, in.ID, in.FileID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &documentOutput{Body: d}, nil
		})

	huma.Register(h.api, h.op("set-file-text", http.MethodPut, "/documents/{id}/files/{fileId}/text", "Type a file's text yourself; empty hands it back to the text service"),
		func(ctx context.Context, in *struct {
			ID     int64 `path:"id"`
			FileID int64 `path:"fileId"`
			Body   struct {
				Text string `json:"text" maxLength:"500000"`
			}
		}) (*documentOutput, error) {
			d, err := h.Catalog.SetFileText(ctx, in.ID, in.FileID, in.Body.Text)
			if err != nil {
				return nil, h.fail(err)
			}
			return &documentOutput{Body: d}, nil
		})

	huma.Register(h.api, h.op("get-open-copy", http.MethodGet, "/documents/{id}/open-copy", "Where OpenAlex knows the document can be read without a subscription, asked afresh"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body openCopyBody }, error) {
			c, err := h.Catalog.OpenCopy(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &struct{ Body openCopyBody }{}
			if c != nil {
				out.Body = openCopyBody{Found: true, Copy: *c}
			}
			return out, nil
		})

	huma.Register(h.api, h.op("add-open-copy", http.MethodPost, "/documents/{id}/open-copy", "Fetch the open copy's PDF and add it to the document's files"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*documentOutput, error) {
			d, err := h.Catalog.AddOpenCopy(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &documentOutput{Body: d}, nil
		})

	// The form's draft as it stands, a title not typed yet among what it may lack: it is filled
	// and handed back, never kept, so it is not held to a saved document's rules.
	fill := func(id, path, summary string) huma.Operation {
		o := h.op(id, http.MethodPost, path, summary)
		o.SkipValidateBody = true
		return o
	}

	citation := fill("lookup-citation", "/lookup/citation", "A paper's blanks filled from a citation exported from a library's database (RIS)")
	citation.MaxBodyBytes = 2 << 20
	huma.Register(h.api, citation, func(_ context.Context, in *struct {
		Body struct {
			RIS      string                `json:"ris" doc:"The exported file's text"`
			Document catalog.DocumentInput `json:"document" doc:"The form as it stands"`
		}
	}) (*documentFillOutput, error) {
		out, err := h.Catalog.FillFromCitation(in.Body.RIS, in.Body.Document)
		if err != nil {
			return nil, h.fail(err)
		}
		return &documentFillOutput{Body: out}, nil
	})

	huma.Register(h.api, h.op("refresh-document", http.MethodPost, "/documents/{id}/refresh", "Look a paper up again by its DOI: its blanks filled, and what Crossref gives otherwise offered"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body *catalog.DocumentSuggestions }, error) {
			out, err := h.Catalog.RefreshDocument(ctx, in.ID)
			if err != nil {
				return nil, h.fail(err)
			}
			return &struct{ Body *catalog.DocumentSuggestions }{Body: out}, nil
		})

	huma.Register(h.api, h.op("find-dois", http.MethodGet, "/lookup/dois", "Papers Crossref has that match a title and author, with their DOIs"),
		func(ctx context.Context, in *struct {
			Title  string `query:"title" maxLength:"500" doc:"The paper's title"`
			Author string `query:"author" maxLength:"300" doc:"Its first author, as written; searched by the surname"`
		}) (*struct {
			Body struct {
				Candidates []lookup.DocProposal `json:"candidates"`
			}
		}, error) {
			found, err := h.Catalog.FindDOIs(ctx, in.Title, in.Author)
			var field *catalog.FieldError
			if err != nil && !errors.As(err, &field) {
				h.Logger.Warn("find doi", "err", err)
				return nil, huma.Error502BadGateway("Crossref could not be reached. Try again in a moment, or type the DOI.")
			} else if err != nil {
				return nil, h.fail(err)
			}
			out := &struct {
				Body struct {
					Candidates []lookup.DocProposal `json:"candidates"`
				}
			}{}
			out.Body.Candidates = found
			return out, nil
		})

	huma.Register(h.api, fill("lookup-doi", "/lookup/doi", "A paper's blanks filled from what Crossref says its DOI is"),
		func(ctx context.Context, in *struct {
			Body struct {
				DOI      string                `json:"doi" doc:"A DOI, bare or as a doi.org link"`
				Document catalog.DocumentInput `json:"document" doc:"The form as it stands"`
			}
		}) (*documentFillOutput, error) {
			out, err := h.Catalog.FillFromDOI(ctx, in.Body.DOI, in.Body.Document)
			if err != nil {
				return nil, h.fail(err)
			}
			return &documentFillOutput{Body: out}, nil
		})
}

type documentFillOutput struct {
	Body *catalog.DocumentFill
}
