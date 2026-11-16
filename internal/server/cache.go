package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"

	"homelibrary/internal/api"
	"homelibrary/internal/auth"
)

// revalidated gives a read the browser may keep its ETag. A GET that answers 200 is held back
// until its body is complete, hashed into a strong ETag, and answered 304 with no body when the
// request's If-None-Match names it, when it is one of two kinds:
//   - a public read, an API GET or the health check that nobody signed in to, sent no-cache;
//   - a list the browser receives whole, which the API marks private, no-cache: the browser
//     keeps it in its own cache alone and asks again each time.
//
// Every other answer passes straight through as private, no-store, as securityHeaders set it,
// whatever its size; a share link's visitor counts as signed in. The change feed is a stream,
// never held back to be hashed, and the app's own files keep the headers the front end's
// handler gives them.
func revalidated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		read := strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/healthz"
		if r.Method != http.MethodGet || !read || isStream(r) {
			next.ServeHTTP(w, r)
			return
		}
		_, signedIn := auth.FromContext(r.Context())
		held := &heldResponse{ResponseWriter: w, public: !signedIn}
		next.ServeHTTP(held, r)
		held.finish(r)
	})
}

func isStream(r *http.Request) bool {
	return r.URL.Path == api.Prefix+"/changes" || strings.Contains(r.Header.Get("Accept"), "text/event-stream")
}

// heldResponse decides as the status is written whether to keep the body back for its ETag:
// a 200 that is public, or that its handler marked as a list to keep. Anything else goes to
// the client as it is written.
type heldResponse struct {
	http.ResponseWriter
	public  bool
	decided bool
	keep    bool
	body    bytes.Buffer
}

func (h *heldResponse) WriteHeader(code int) {
	if h.decided {
		return
	}
	h.decided = true
	h.keep = code == http.StatusOK && (h.public || h.Header().Get("Cache-Control") == api.KeptList)
	if !h.keep {
		// A list that failed is an error like any other, which no browser keeps.
		if h.Header().Get("Cache-Control") == api.KeptList {
			h.Header().Set("Cache-Control", "private, no-store")
		}
		h.ResponseWriter.WriteHeader(code)
	}
}

func (h *heldResponse) Write(b []byte) (int, error) {
	h.WriteHeader(http.StatusOK)
	if !h.keep {
		return h.ResponseWriter.Write(b)
	}
	return h.body.Write(b)
}

// Unwrap lets http.ResponseController reach the connection, for a read deadline.
func (h *heldResponse) Unwrap() http.ResponseWriter { return h.ResponseWriter }

// finish sends a kept body under its ETag, or 304 when the request holds it already.
func (h *heldResponse) finish(r *http.Request) {
	if !h.keep {
		return
	}
	sum := sha256.Sum256(h.body.Bytes())
	tag := `"` + base64.RawURLEncoding.EncodeToString(sum[:]) + `"`
	dst := h.Header()
	dst.Set("ETag", tag)
	if h.public {
		dst.Set("Cache-Control", "no-cache")
	}
	if matches(r.Header.Get("If-None-Match"), tag) {
		for _, k := range []string{"Content-Type", "Content-Length", "Content-Encoding"} {
			dst.Del(k)
		}
		h.ResponseWriter.WriteHeader(http.StatusNotModified)
		return
	}
	h.ResponseWriter.WriteHeader(http.StatusOK)
	_, _ = h.ResponseWriter.Write(h.body.Bytes()) // a client that hung up asks again
}

// matches reports whether an If-None-Match header names tag, comparing weakly as RFC 9110
// has it for this header: a W/ prefix on either side is ignored.
func matches(header, tag string) bool {
	if strings.TrimSpace(header) == "*" {
		return true
	}
	want := strings.TrimPrefix(tag, "W/")
	for t := range strings.SplitSeq(header, ",") {
		if strings.TrimPrefix(strings.TrimSpace(t), "W/") == want {
			return true
		}
	}
	return false
}
