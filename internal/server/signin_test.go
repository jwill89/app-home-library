package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"homelibrary/internal/api"
	"homelibrary/internal/auth"
	"homelibrary/internal/db"
)

// setupCode makes the owner and a fresh code for them, as `homelibrary setup-code` does.
func setupCode(t *testing.T, a *App) string {
	t.Helper()
	ctx := context.Background()
	q := db.New(a.db.Write)
	u, err := auth.EnsureOwner(ctx, q, "James")
	if err != nil {
		t.Fatal(err)
	}
	code, _, err := auth.NewSetupCode(ctx, q, u.ID, 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

// options is a ceremony's first answer, as far as the tests read it.
type options struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
	} `json:"publicKey"`
}

func TestRedeemSetupCode(t *testing.T) {
	a := newApp(t)
	code := setupCode(t, a)
	device := &client{t: t, a: a}
	device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"ABCD-EFGH"}`, http.StatusUnauthorized)
	device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":""}`, http.StatusUnprocessableEntity)
	device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"   "}`, http.StatusUnprocessableEntity)
	before := device.cookie
	w := device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"`+code+`"}`, http.StatusNoContent)
	if device.cookie == "" || device.cookie == before {
		t.Fatal("redeeming a code kept the session's token")
	}
	if left := lifetime(w); left > 11*time.Minute {
		t.Fatalf("a device that only took a code keeps its session %s", left)
	}
	// The code's ten minutes hold the passkey's first step, made for the name asked on.
	opts := decode[options](t, device.expect(http.MethodPost, "/api/v1/passkeys/begin", "", http.StatusOK))
	if opts.PublicKey.RP.ID != "localhost" || opts.PublicKey.Challenge == "" {
		t.Fatalf("passkey options %+v", opts)
	}
	device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"`+code+`"}`, http.StatusUnauthorized)
	// The code makes a passkey; it signs no one in by itself.
	if who := decode[struct{ Role string }](t, device.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "none" {
		t.Fatalf("a redeemed code made the device %s", who.Role)
	}

	// The owner who takes a code stays signed in for the session's whole lifetime.
	me := owner(t, a)
	w = me.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"`+setupCode(t, a)+`"}`, http.StatusNoContent)
	if left := lifetime(w); left < 24*time.Hour {
		t.Fatalf("taking a code cut the owner's session to %s", left)
	}
	if who := decode[struct{ Role string }](t, me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "owner" {
		t.Fatalf("the owner who took a code is %s", who.Role)
	}
}

// lifetime is how long the session cookie an answer set has left.
func lifetime(w *httptest.ResponseRecorder) time.Duration {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "hl_session" {
			return time.Until(ck.Expires)
		}
	}
	return 0
}

func TestBeginPasskeyGuard(t *testing.T) {
	a := newApp(t)
	(&client{t: t, a: a}).expect(http.MethodPost, "/api/v1/passkeys/begin", "", http.StatusUnauthorized)
	(&client{t: t, a: a}).expect(http.MethodPost, "/api/v1/passkeys", `{"credential":{}}`, http.StatusUnauthorized)
	_, token, err := auth.NewShare(context.Background(), a.db, "Mom", time.Now().Add(time.Hour), scope("all", true, "none"))
	if err != nil {
		t.Fatal(err)
	}
	visitor := &client{t: t, a: a}
	visitor.expect(http.MethodGet, "/s/"+token, "", http.StatusSeeOther)
	visitor.expect(http.MethodPost, "/api/v1/passkeys/begin", "", http.StatusForbidden)
	visitor.expect(http.MethodPost, "/api/v1/passkeys", `{"credential":{}}`, http.StatusForbidden)
	me := owner(t, a)
	me.expect(http.MethodPost, "/api/v1/passkeys/begin", "", http.StatusOK)
	// A name over sixty characters is refused, not cut short.
	me.expect(http.MethodPost, "/api/v1/passkeys", `{"name":"`+strings.Repeat("a", 61)+`","credential":{}}`, http.StatusUnprocessableEntity)
	// The challenge answers once: an answer that isn't a passkey spends it.
	refused := decode[struct{ Detail string }](t, me.expect(http.MethodPost, "/api/v1/passkeys", `{"credential":{"id":"x"}}`, http.StatusBadRequest))
	again := decode[struct{ Detail string }](t, me.expect(http.MethodPost, "/api/v1/passkeys", `{"credential":{"id":"x"}}`, http.StatusBadRequest))
	if !strings.Contains(refused.Detail, "not accepted") || !strings.Contains(again.Detail, "No passkey is being made") {
		t.Fatalf("refused %q, then %q", refused.Detail, again.Detail)
	}
}

func TestPasskeySignIn(t *testing.T) {
	a := newApp(t)
	device := &client{t: t, a: a}
	device.expect(http.MethodPost, "/api/v1/auth/passkey/finish", `{"credential":{"id":"x"}}`, http.StatusBadRequest)
	opts := decode[options](t, device.expect(http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusOK))
	if opts.PublicKey.Challenge == "" {
		t.Fatalf("sign-in options %+v", opts)
	}
	w := device.expect(http.MethodPost, "/api/v1/auth/passkey/finish", `{"credential":{"id":"x","type":"public-key"}}`, http.StatusUnauthorized)
	if p := decode[struct{ Detail string }](t, w); p.Detail == "" {
		t.Fatalf("a refused sign-in says nothing: %s", w.Body.String())
	}
	// The refused answer spent the challenge.
	device.expect(http.MethodPost, "/api/v1/auth/passkey/finish", `{"credential":{"id":"x"}}`, http.StatusBadRequest)
}

func TestSignOut(t *testing.T) {
	a := newApp(t)
	me := owner(t, a)
	if who := decode[struct{ Role string }](t, me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "owner" {
		t.Fatalf("signed in as %s", who.Role)
	}
	token := me.cookie
	me.expect(http.MethodPost, "/api/v1/auth/logout", "", http.StatusNoContent)
	if who := decode[struct{ Role string }](t, me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "none" {
		t.Fatalf("signed out as %s", who.Role)
	}
	// The session is gone from the store, not only from the browser.
	me.cookie = token
	if who := decode[struct{ Role string }](t, me.expect(http.MethodGet, "/api/v1/me", "", http.StatusOK)); who.Role != "none" {
		t.Fatalf("the old cookie still signs in as %s", who.Role)
	}
}

func TestSignInLimit(t *testing.T) {
	a := newApp(t)
	device := &client{t: t, a: a}
	for range 20 {
		device.expect(http.MethodPost, "/api/v1/auth/logout", "", http.StatusNoContent)
	}
	for range 10 {
		device.expect(http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusOK)
	}
	w := device.expect(http.MethodPost, "/api/v1/auth/passkey/begin", "", http.StatusTooManyRequests)
	if wait, err := strconv.Atoi(w.Header().Get("Retry-After")); err != nil || wait < 1 {
		t.Errorf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	device.expect(http.MethodPost, "/api/v1/auth/setup", `{"code":"ABCD-EFGH"}`, http.StatusTooManyRequests)
	device.expect(http.MethodPost, "/api/v1/auth/logout", "", http.StatusNoContent)
}

func TestSignInDocument(t *testing.T) {
	raw, err := api.Spec()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string                     `json:"operationId"`
			Responses   map[string]json.RawMessage `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		path, method, id string
		statuses         []string
	}{
		{"/auth/passkey/begin", "post", "begin-passkey-sign-in", []string{"200", "429"}},
		{"/auth/passkey/finish", "post", "finish-passkey-sign-in", []string{"200", "400", "401", "429"}},
		{"/auth/logout", "post", "sign-out", []string{"204"}},
		{"/auth/setup", "post", "redeem-setup-code", []string{"204", "401", "422", "429"}},
		{"/passkeys/begin", "post", "begin-passkey", []string{"200", "401", "403"}},
		{"/passkeys", "post", "add-passkey", []string{"201", "400", "401", "403", "409", "422"}},
	}
	for _, w := range want {
		op, ok := doc.Paths[w.path][w.method]
		if !ok || op.OperationID != w.id {
			t.Errorf("%s %s: %q, want %s", w.method, w.path, op.OperationID, w.id)
			continue
		}
		for _, s := range w.statuses {
			if _, ok := op.Responses[s]; !ok {
				t.Errorf("%s answers no %s", w.id, s)
			}
		}
		if !slices.Contains(w.statuses, "200") && !slices.Contains(w.statuses, "201") {
			if _, ok := op.Responses["200"]; ok {
				t.Errorf("%s answers 200 with no content", w.id)
			}
		}
	}
}
