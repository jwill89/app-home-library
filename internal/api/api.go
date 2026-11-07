// Package api registers the JSON API: typed handlers with validation and a generated
// OpenAPI document the frontend's client is built from.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/covers"
	"homelibrary/internal/db"
	"homelibrary/internal/events"
	"homelibrary/internal/files"
	"homelibrary/internal/lookup"
)

// Prefix is the path every operation lives under.
const Prefix = "/api/v1"

// Deps is everything the handlers need.
type Deps struct {
	DB      *db.DB
	Catalog *catalog.Service
	Lookup  *lookup.Service
	Feed    *events.Feed
	Logger  *slog.Logger
}

type handlers struct {
	Deps
	api huma.API
}

// New registers every operation on the mux and returns the API for its OpenAPI document.
func New(mux *http.ServeMux, d Deps) huma.API {
	cfg := huma.DefaultConfig("Home Library", "1")
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	// No "$schema" link in every answer: nothing reads the schemas from the server.
	cfg.CreateHooks = nil
	cfg.Servers = []*huma.Server{{URL: Prefix}}
	cfg.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"session": {Type: "apiKey", In: "cookie", Name: "hl_session"},
	}
	api := humago.NewWithPrefix(mux, Prefix, cfg)
	h := &handlers{Deps: d, api: api}
	h.registerAccount()
	h.registerBooks()
	h.registerCopies()
	h.registerLoans()
	h.registerCourses()
	h.registerLabels()
	h.registerDocuments()
	h.registerShelves()
	h.registerLookup()
	h.registerScans()
	h.registerImports()
	h.registerShares()
	h.registerExport()
	h.registerSeries()
	h.registerReading()
	h.registerGaps()
	h.registerSuggestions()
	h.registerCalendar()
	h.registerChanges()
	return api
}

// Spec renders the OpenAPI document without a database or a server.
func Spec() ([]byte, error) {
	api := New(http.NewServeMux(), Deps{})
	return json.MarshalIndent(api.OpenAPI(), "", "  ")
}

var secured = []map[string][]string{{"session": {}}}

// op is an operation only the owner may call, which is all but the session check and reading.
func (h *handlers) op(id, method, path, summary string) huma.Operation {
	return huma.Operation{
		OperationID: id,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Security:    secured,
		Middlewares: huma.Middlewares{auth.RequireOwner(h.api)},
	}
}

// readOp is an operation that reads the catalog, open to the owner and to a share link's
// visitor. What a visitor may not see, the handler leaves out.
func (h *handlers) readOp(id, path, summary string) huma.Operation {
	o := h.op(id, http.MethodGet, path, summary)
	o.Middlewares = huma.Middlewares{auth.RequireReader(h.api)}
	return o
}

// fail turns a catalog error into the answer the owner should see, and logs the rest.
func (h *handlers) fail(err error) error {
	var field *catalog.FieldError
	var dup *catalog.DuplicateError
	var conflict *catalog.ConflictError
	switch {
	case errors.As(err, &field):
		return huma.Error422UnprocessableEntity(field.Message, &huma.ErrorDetail{Location: "body." + field.Field, Message: field.Message})
	case errors.As(err, &dup):
		return huma.Error409Conflict(dup.Error())
	case errors.As(err, &conflict):
		return huma.Error409Conflict(conflict.Message)
	case errors.Is(err, catalog.ErrNotFound):
		return huma.Error404NotFound("not found")
	case errors.Is(err, covers.ErrTooLarge):
		return huma.Error422UnprocessableEntity("That image is too large. Use one under 12 MB.")
	case errors.Is(err, covers.ErrTooSmall):
		return huma.Error422UnprocessableEntity("That image is too small to be a cover.")
	case errors.Is(err, covers.ErrUnsupported):
		return huma.Error422UnprocessableEntity("That is not a JPEG, PNG, or WebP image.")
	case errors.Is(err, files.ErrUnsupported):
		return huma.Error422UnprocessableEntity("That is not a PDF or a JPEG, PNG, or WebP image.")
	case errors.Is(err, files.ErrTooLarge):
		return huma.Error422UnprocessableEntity("That file is too large. Use one under 40 MB, or split the PDF.")
	}
	h.Logger.Error("request failed", "err", err)
	return huma.Error500InternalServerError("something went wrong")
}
