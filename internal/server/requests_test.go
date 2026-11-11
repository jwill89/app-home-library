package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPagesAreOnlyRead(t *testing.T) {
	a := newApp(t)
	c := &client{t: t, a: a}
	// An installed app left open from before sign-in moved into the API posts here.
	w := c.expect(http.MethodPost, "/auth/passkey/login/begin", "", http.StatusMethodNotAllowed)
	if allow := w.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("Allow %q", allow)
	}
	c.expect(http.MethodPost, "/sign-in", "", http.StatusMethodNotAllowed)
	c.expect(http.MethodPost, "/s/not-a-token", "", http.StatusMethodNotAllowed)
	c.expect(http.MethodPost, "/api/v1/nowhere", "", http.StatusNotFound)
}

func TestRequestLogHidesLinkTokens(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	h := requestLog(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for _, p := range []string{"/s/share-secret", "/calendar/calendar-secret.ics", "/library"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://"+host+p, nil))
	}
	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Fatalf("the log holds a link's token:\n%s", out)
	}
	share := sha256.Sum256([]byte("share-secret"))
	calendar := sha256.Sum256([]byte("calendar-secret"))
	for _, want := range []string{
		"path=/s/#" + hex.EncodeToString(share[:4]),
		"path=/calendar/#" + hex.EncodeToString(calendar[:4]),
		"path=/library",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %s:\n%s", want, out)
		}
	}
}
