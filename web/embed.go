// Package web embeds the built frontend and serves it as a single-page app.
package web

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// noncePlaceholder is what vite.config.ts stamps into index.html; every response replaces it.
const noncePlaceholder = "__CSP_NONCE__"

// coverHosts are where the lookup's cover candidates and search thumbnails are shown from
// before one is chosen and stored here. Open Library's covers redirect to the Internet
// Archive, Google's to its content hosts, and Hardcover's are on its own asset host.
const coverHosts = "https://covers.openlibrary.org https://archive.org https://*.archive.org https://books.google.com https://*.googleusercontent.com https://assets.hardcover.app"

// ContentSecurityPolicy is the policy for every response. A nonce admits the style
// elements PrimeVue injects at runtime, and only on the app shell that carries it.
func ContentSecurityPolicy(nonce string) string {
	styles := "'self'"
	if nonce != "" {
		styles += " 'nonce-" + nonce + "'"
	}
	return "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src " + styles +
		"; img-src 'self' data: blob: " + coverHosts + "; connect-src 'self'" +
		"; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
}

// WorkerPolicy is the policy for the cover-detection worker's script alone. OpenCV's bindings
// build functions with `new Function`, and it loads its WebAssembly from a data: address
// inside its own script; every page's policy forbids both. A dedicated worker takes the
// policy its own script is served with, so this one may and no page need. The script is
// cached for good under its hashed name with this header, so a change here must come with a
// change to the worker's code, which gives it a new name.
const WorkerPolicy = "default-src 'self'; script-src 'self' 'unsafe-eval' 'wasm-unsafe-eval'; connect-src 'self' data:"

// isDetectWorker reports whether a built file is the cover-detection worker (app/detect.worker.ts).
func isDetectWorker(name string) bool {
	return strings.HasPrefix(name, "assets/detect.worker-") && strings.HasSuffix(name, ".js")
}

// Handler serves built assets and falls back to index.html for app routes. Hashed files
// under assets/ are immutable; the shell and the service worker are never cached.
func Handler() http.Handler {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	static := http.FileServerFS(files)
	shell, _ := fs.ReadFile(files, "index.html") // nil until the frontend is built

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, err := fs.Stat(files, name); err == nil && !info.IsDir() {
			switch {
			case strings.HasPrefix(name, "assets/"):
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				if isDetectWorker(name) {
					w.Header().Set("Content-Security-Policy", WorkerPolicy)
				}
			case strings.HasPrefix(name, "icons/"):
				w.Header().Set("Cache-Control", "public, max-age=86400")
			case name == "sw.js":
				w.Header().Set("Cache-Control", "no-cache")
			}
			static.ServeHTTP(w, r)
			return
		}
		// A hashed file that is gone belongs to an older build; answering with the shell
		// would hand a script loader a page of HTML.
		if strings.HasPrefix(name, "api/") || strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
		if shell == nil {
			http.Error(w, "frontend not built: run task build:web", http.StatusServiceUnavailable)
			return
		}
		nonce := newNonce()
		h := w.Header()
		h.Set("Content-Security-Policy", ContentSecurityPolicy(nonce))
		h.Set("Content-Type", "text/html; charset=utf-8")
		h.Set("Cache-Control", "no-cache")
		_, _ = w.Write(bytes.ReplaceAll(shell, []byte(noncePlaceholder), []byte(nonce)))
	})
}

func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand never fails on the platforms this runs on
	return base64.StdEncoding.EncodeToString(b[:])
}
