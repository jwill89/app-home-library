// Package server assembles the HTTP handler: health, sign-in, covers, the API, and the
// embedded frontend, with logging, a host check, security headers, CSRF protection, rate
// limits, and sessions around them.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"homelibrary/internal/api"
	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/config"
	"homelibrary/internal/covers"
	"homelibrary/internal/db"
	"homelibrary/internal/events"
	"homelibrary/internal/files"
	"homelibrary/internal/lookup"
	"homelibrary/internal/ratelimit"
	"homelibrary/internal/reqid"
	"homelibrary/internal/version"
	"homelibrary/web"
)

// UserAgent is how every request the app makes names it, as Open Library asks.
func UserAgent() string {
	return "HomeLibrary/" + version.Version + " (+https://books.jamescwill.com)"
}

// App owns the resources behind the handler.
type App struct {
	cfg      config.Config
	logger   *slog.Logger
	db       *db.DB
	schema   int64
	sessions *scs.SessionManager
	catalog  *catalog.Service
	feed     *events.Feed
	handler  http.Handler
}

// Services opens what both the server and the maintenance commands use: the database,
// migrated, and the catalog over it.
func Services(ctx context.Context, cfg config.Config, logger *slog.Logger) (*db.DB, int64, *catalog.Service, error) {
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return nil, 0, nil, err
	}
	schema, err := db.Migrate(ctx, database.Write)
	if err != nil {
		return nil, 0, nil, errors.Join(err, database.Close())
	}
	store, err := covers.NewStore(cfg.MediaDir)
	if err != nil {
		return nil, 0, nil, errors.Join(err, database.Close())
	}
	docs, err := files.NewStore(cfg.DocsDir)
	if err != nil {
		return nil, 0, nil, errors.Join(err, database.Close())
	}
	client := outbound()
	books := lookup.New(lookup.Options{
		UserAgent:      UserAgent(),
		GoogleKey:      cfg.GoogleKey,
		HardcoverToken: cfg.HardcoverToken,
		Cache:          &catalog.LookupCache{DB: database, Logger: logger},
		Logger:         logger,
		HTTPClient:     client,
	})
	svc := &catalog.Service{DB: database, Covers: store, Files: docs, Lookup: books, HTTP: client, UserAgent: UserAgent(), Logger: logger}
	return database, schema, svc, nil
}

// New opens everything and wires every route.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	database, schema, svc, err := Services(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	if cfg.GoogleKey == "" {
		logger.Info("google books not configured: lookups use the Library of Congress and Open Library")
	}
	if cfg.HardcoverToken == "" {
		logger.Info("hardcover not configured: covers come from Open Library alone")
	}
	a := &App{cfg: cfg, logger: logger, db: database, schema: schema, catalog: svc}
	a.sessions = auth.NewSessionManager(database.Write, database.Read, cfg.Secure())
	a.sessions.ErrorFunc = func(w http.ResponseWriter, _ *http.Request, err error) {
		logger.Error("session", "err", err)
		http.Error(w, "session error", http.StatusInternalServerError)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.health)
	(&auth.Passkeys{Sessions: a.sessions, DB: database, Origin: cfg.Origin, Logger: logger}).Register(mux)
	(&auth.ShareLinks{Sessions: a.sessions, DB: database, Logger: logger}).Register(mux)
	mux.Handle("GET /media/{item}/{file}", signedIn(svc.Covers.Handler()))
	mux.HandleFunc("GET /calendar/{file}", a.calendar)
	a.feed = events.New(database.Read, database.Write, logger)
	api.New(mux, api.Deps{DB: database, Catalog: svc, Lookup: svc.Lookup, Feed: a.feed, Logger: logger})
	mux.Handle("/", web.Handler())

	signIn := ratelimit.New(20, 10)
	writes := ratelimit.New(240, 120)
	visits := ratelimit.New(120, 60)
	h := writes.Middleware(writeKey)(routed(mux))
	h = visits.Middleware(visitKey)(h)
	h = a.sessions.LoadAndSave(auth.Resolve(a.sessions, database.Read, cfg.Local(), h))
	h = signIn.Middleware(signInKey)(h)
	h = http.NewCrossOriginProtection().Handler(h)
	h = securityHeaders(h)
	h = knownHost(cfg, h)
	a.handler = requestLog(logger, h)
	return a, nil
}

// Handler is the root handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Close releases the database.
func (a *App) Close() error {
	return a.db.Close()
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	status, code := "ok", http.StatusOK
	if err := a.db.Read.PingContext(r.Context()); err != nil {
		status, code = "database unreachable", http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": status, "version": version.Version, "commit": version.Commit,
		"buildDate": version.BuildDate, "schema": strconv.FormatInt(a.schema, 10),
	})
}

// knownHost answers only on the configured names. Every name-dependent thing (cookies, the
// passkey's relying party) is built from the configured origin for the name, so a request
// for any other name is refused before it reaches them. The health check answers on any,
// since the deploy script asks for it by address.
func knownHost(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := cfg.Origin(r.Host); !ok && r.URL.Path != "/healthz" {
			http.Error(w, "unknown host", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// signedIn keeps the covers, which the owner may have photographed, to the owner and the
// visitors the owner has given a link.
func signedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := auth.FromContext(r.Context()); !ok {
			http.Error(w, "sign in to continue", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// signInKey covers the routes a stranger can try secrets on: passkeys, setup codes, share
// links, and the calendar link.
func signInKey(r *http.Request) string {
	if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/s/") || strings.HasPrefix(r.URL.Path, "/calendar/") {
		return ratelimit.ClientKey(r)
	}
	return ""
}

// visitKey holds each visitor on a link to a reader's pace, so a link passed around cannot
// become a load on the server. Covers are cached for good and are not counted.
func visitKey(r *http.Request) string {
	p, ok := auth.FromContext(r.Context())
	if !ok || p.Owner() || !strings.HasPrefix(r.URL.Path, "/api/") {
		return ""
	}
	return "share:" + strconv.FormatInt(p.ShareID, 10) + ":" + ratelimit.ClientKey(r)
}

func writeKey(r *http.Request) string {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || !strings.HasPrefix(r.URL.Path, "/api/") {
		return ""
	}
	return "ip:" + ratelimit.ClientKey(r)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", web.ContentSecurityPolicy(""))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		// The camera is for scanning barcodes and photographing covers, on this site only.
		h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		// The library is private: no search engine is to list any of it.
		h.Set("X-Robots-Tag", "noindex, nofollow")
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/auth/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type servedKey struct{}

// routed tells the request log which route the mux matched.
func routed(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
		if route, ok := r.Context().Value(servedKey{}).(*string); ok {
			*route = r.Pattern
		}
	})
}

// requestLog names the matched route, not the path, above debug: a path can carry a search.
func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := reqid.New()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		route := ""
		ctx := context.WithValue(r.Context(), servedKey{}, &route)
		next.ServeHTTP(sw, r.WithContext(ctx))
		attrs := []any{"id", id, "method", r.Method, "route", route, "status", sw.status, "ms", time.Since(start).Milliseconds()}
		if logger.Enabled(ctx, slog.LevelDebug) {
			attrs = append(attrs, "path", r.URL.Path)
		}
		logger.Info("request", attrs...)
	})
}

// Listen runs the server and the background lookup until the context is cancelled.
func (a *App) Listen(ctx context.Context) error {
	srv := &http.Server{Addr: a.cfg.Addr, Handler: a.handler, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	go a.catalog.RunEnricher(ctx)
	go a.catalog.RunScans(ctx)
	go a.feed.RunPruner(ctx)
	a.logger.Info("listening", "addr", a.cfg.Addr, "origins", a.cfg.Origins, "version", version.Version, "schema", a.schema)
	select {
	case err := <-errc:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}
	// The open streams end first: each is a request the shutdown would otherwise wait out.
	a.feed.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil {
		a.logger.Warn("shutdown cut requests short", "err", err)
	}
	return nil
}

// calendar serves the loans' due dates at the calendar link, /calendar/{token}.ics, to a
// phone's calendar, which cannot sign in. A token that is not the link's finds nothing.
func (a *App) calendar(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSuffix(r.PathValue("file"), ".ics")
	ok, err := auth.CalendarLinkFor(r.Context(), db.New(a.db.Write), token)
	if err != nil {
		a.logger.Error("calendar link", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := a.catalog.LoansCalendar(r.Context(), time.Now())
	if err != nil {
		a.logger.Error("calendar", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/calendar; charset=utf-8")
	h.Set("Content-Disposition", `inline; filename="loans.ics"`)
	h.Set("Cache-Control", "no-store")
	_, _ = w.Write(body) // a calendar that hangs up early fetches again on its next refresh
}
