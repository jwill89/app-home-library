// Package api registers the JSON API: typed handlers with validation and a generated
// OpenAPI document the frontend's client is built from.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
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
	// Sessions is the session manager whose session every request's context carries (the
	// server's LoadAndSave), so signing in and out happen on it.
	Sessions *scs.SessionManager
	Logger   *slog.Logger
	// Level is the server's log level, which the owner may change while it runs; StartLevel is
	// HL_LOG_LEVEL's, which a restart goes back to.
	Level      *slog.LevelVar
	StartLevel slog.Level
}

type handlers struct {
	Deps
	api      huma.API
	passkeys *auth.Passkeys
}

// Request is what the server knows of a request that an operation can't read for itself.
type Request struct {
	// Site is the configured origin the request came on, scheme and host: the relying party
	// a passkey belongs to.
	Site string
}

type requestKey struct{}

// WithRequest returns ctx carrying r; the server's host check adds it to every request on a
// name the site answers on.
func WithRequest(ctx context.Context, r Request) context.Context {
	return context.WithValue(ctx, requestKey{}, r)
}

// requestOf is the Request ctx carries, or none.
func requestOf(ctx context.Context) Request {
	r, _ := ctx.Value(requestKey{}).(Request)
	return r
}

// New registers every operation on the mux and returns the API for its OpenAPI document.
func New(mux *http.ServeMux, d Deps) huma.API {
	cfg := huma.DefaultConfig("Home Library", "1")
	cfg.Info.Description = guardsExplained
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
	h := &handlers{Deps: d, api: api, passkeys: &auth.Passkeys{Sessions: d.Sessions, DB: d.DB, Logger: d.Logger}}
	h.registerSignIn()
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
	h.registerLogLevel()
	describeGuards(api.OpenAPI())
	return api
}

// Spec renders the OpenAPI document without a database or a server.
func Spec() ([]byte, error) {
	api := New(http.NewServeMux(), Deps{})
	return json.MarshalIndent(api.OpenAPI(), "", "  ")
}

var secured = []map[string][]string{{"session": {}}}

// The guards an operation's description opens with, in the API standard's terms: who may
// call it. guardsExplained says what each means here.
const (
	guardKey    = "guard"
	guardPublic = "Guard: public."
	guardAdmin  = "Guard: admin."
	guardAuth   = "Guard: auth."

	guardsExplained = "Each operation's description opens with its guard. public: anyone. " +
		"auth: anyone signed in, the owner or a share link's visitor, who sees only what the link shows. " +
		"admin: the owner alone."
)

// KeptList is the Cache-Control of a list the browser receives whole: kept in the browser's own
// cache alone, and asked for again each time, so the server's ETag can answer an unchanged one
// with an empty 304 (the server's revalidated). Every other answer to someone signed in is
// private, no-store.
const KeptList = "private, no-cache"

// op is an operation only the owner may call, which is all but the session check and reading.
// A list (an operation named list-...) is one the browser receives whole, and may keep.
func (h *handlers) op(id, method, path, summary string) huma.Operation {
	o := huma.Operation{
		OperationID: id,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Security:    secured,
		Middlewares: huma.Middlewares{auth.RequireOwner(h.api)},
		Metadata:    map[string]any{guardKey: guardAdmin},
	}
	return keptIfList(o)
}

// readOp is an operation that reads the catalog, open to the owner and to a share link's
// visitor. What a visitor may not see, the handler leaves out.
func (h *handlers) readOp(id, path, summary string) huma.Operation {
	o := h.op(id, http.MethodGet, path, summary)
	o.Middlewares = huma.Middlewares{auth.RequireReader(h.api)}
	o.Metadata[guardKey] = guardAuth
	return keptIfList(o)
}

// keptIfList marks a list's answer as one the browser may keep, after its guard has let the
// caller through.
func keptIfList(o huma.Operation) huma.Operation {
	if o.Method == http.MethodGet && strings.HasPrefix(o.OperationID, "list-") {
		o.Middlewares = append(o.Middlewares, func(ctx huma.Context, next func(huma.Context)) {
			ctx.SetHeader("Cache-Control", KeptList)
			next(ctx)
		})
	}
	return o
}

// describeGuards opens every operation's description with its guard, so the document says
// who may call each: the owner's and the readers' as op and readOp made them, and public for
// the rest, the sign-in operations and the session check.
func describeGuards(doc *huma.OpenAPI) {
	for _, item := range doc.Paths {
		for _, o := range []*huma.Operation{item.Get, item.Put, item.Post, item.Patch, item.Delete} {
			if o == nil {
				continue
			}
			guard, ok := o.Metadata[guardKey].(string)
			if !ok {
				guard = guardPublic
			}
			o.Description = strings.TrimSpace(guard + " " + o.Description)
		}
	}
}

// fail turns a catalog error into the answer the owner should see, and logs the rest.
func (h *handlers) fail(err error) error {
	var field *catalog.FieldError
	var dup *catalog.DuplicateError
	var conflict *catalog.ConflictError
	var moved *catalog.MovedError
	switch {
	case errors.As(err, &field):
		return huma.Error422UnprocessableEntity(field.Message, &huma.ErrorDetail{Location: "body." + field.Field, Message: field.Message})
	case errors.As(err, &dup):
		return huma.Error409Conflict(dup.Error())
	case errors.As(err, &conflict):
		return huma.Error409Conflict(conflict.Message)
	case errors.As(err, &moved):
		// Named by its field, so a form tells this refusal from any other conflict.
		return huma.Error409Conflict(moved.Error(), &huma.ErrorDetail{Location: "body.updatedAt", Message: moved.Error()})
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
