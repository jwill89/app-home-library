// Package auth signs the owner in with passkeys and keeps the session. The site answers on
// more than one name, and a passkey belongs to the name it was made on, so every ceremony
// takes its relying party from the name the request came on.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/catalog"
	"homelibrary/internal/db"
)

const (
	sessionUser    = "user_id"
	sessionPasskey = "passkey"    // the credential the owner signed in with: removing it signs them out
	sessionSetup   = "setup_user" // a setup code was redeemed: the next passkey made signs in
	// sessionSetupUntil is when a redeemed code's leave to make a passkey ends, in Unix
	// seconds: the session's own deadline moves with every sign-in begun, so it can't be that.
	sessionSetupUntil = "setup_until"

	// devPasskey stands for a passkey in a dev-session's sign-in, honoured only while every
	// origin the server answers on is this machine.
	devPasskey = "development"

	// unfinished is how long a session lasts that is only part way into signing in: a sign-in
	// begun, or a setup code taken. Signing in gives it the whole lifetime.
	unfinished = 10 * time.Minute
)

// NewSessionManager keeps sessions in the database. A sign-in lasts half a year: the
// devices are the owner's own, and signing in again is a fingerprint. Tokens are kept hashed,
// so a copy of the database (a backup) signs no one in. A session is read on the read pool:
// every request reads it, and only a change waits for the one write connection.
func NewSessionManager(write, read *sql.DB, secure bool) *scs.SessionManager {
	sm := scs.New()
	sm.Store = splitStore{
		read:  sqlite3store.NewWithConfig(read, sqlite3store.Config{TableName: "sessions"}),
		write: sqlite3store.NewWithConfig(write, sqlite3store.Config{CleanUpInterval: time.Hour, TableName: "sessions"}),
	}
	sm.HashTokenInStore = true
	sm.Lifetime = 180 * 24 * time.Hour
	// Over HTTPS the cookie takes the __Host- name, which a browser keeps from any other
	// site's setting it, as another app under jamescwill.com could otherwise.
	sm.Cookie.Name = "hl_session"
	if secure {
		sm.Cookie.Name = "__Host-hl_session"
	}
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = secure
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Persist = true
	return sm
}

// Principal is who a request is from: the owner, or a visitor who came by a share link.
type Principal struct {
	UserID  int64          // the owner; 0 for a visitor
	ShareID int64          // the link a visitor came by; 0 for the owner
	Name    string         // the owner's name, whoever is asking
	Scope   *catalog.Scope // what a visitor's link shows; nil for the owner, who sees everything
}

// Owner reports whether the request is the owner's own.
func (p Principal) Owner() bool { return p.UserID > 0 }

type principalKey struct{}

// FromContext returns who the request is from, if anyone.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Resolve puts who the request is from in its context: the signed-in owner, while the
// passkey they signed in with is still theirs, or a visitor whose share link is still running.
// A passkey removed, or a link removed or ended, is forgotten by the session. local says every
// origin is this machine, where a dev-session's sign-in counts.
func Resolve(sm *scs.SessionManager, read *sql.DB, local bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := db.New(read)
		if id := sm.GetInt64(ctx, sessionUser); id > 0 {
			u, err := signedIn(ctx, q, id, sm.GetString(ctx, sessionPasskey), local)
			switch {
			case err == nil:
				r = r.WithContext(context.WithValue(ctx, principalKey{}, Principal{UserID: u.ID, Name: u.Name}))
			case errors.Is(err, errSignedOut):
				sm.Remove(ctx, sessionUser)
				sm.Remove(ctx, sessionPasskey)
			}
			// Any other error is a read that failed (the request given up by the browser, the
			// database busy): it proves nothing, so the session stays as it is, and only this
			// request goes unanswered as the owner's.
		} else if h := sm.GetString(ctx, sessionShare); h != "" {
			// The link is known by its token's hash, which no later link can share; a link's
			// number is used again once the newest is removed. What it shows is read with it,
			// so a change holds from the visitor's next request; a link whose choices cannot
			// be read lets no one in.
			share, err := q.ShareByToken(ctx, db.ShareByTokenParams{TokenHash: h, Now: stamp(time.Now())})
			if errors.Is(err, sql.ErrNoRows) {
				sm.Remove(ctx, sessionShare)
			} else if err == nil {
				if scope, err := scopeOf(ctx, q, share); err == nil {
					name := ""
					if u, err := q.GetFirstUser(ctx); err == nil {
						name = u.Name
					}
					r = r.WithContext(context.WithValue(ctx, principalKey{}, Principal{ShareID: share.ID, Name: name, Scope: &scope}))
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// RequireOwner is operation middleware for everything a visitor may not do, which is all but
// reading the catalog.
func RequireOwner(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		p, ok := FromContext(ctx.Context())
		switch {
		case !ok:
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "sign in to continue")
		case !p.Owner():
			huma.WriteErr(api, ctx, http.StatusForbidden, "a shared view is read-only")
		default:
			next(ctx)
		}
	}
}

// RequireReader is operation middleware for reading the catalog: the owner, or a visitor.
func RequireReader(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if _, ok := FromContext(ctx.Context()); !ok {
			huma.WriteErr(api, ctx, http.StatusUnauthorized, "sign in, or open the link you were given")
			return
		}
		next(ctx)
	}
}

// Registrant is who may make a passkey now: the signed-in owner, or a device that redeemed a
// setup code in the last ten minutes; 0 for anyone else.
func Registrant(ctx context.Context, sm *scs.SessionManager) int64 {
	if p, ok := FromContext(ctx); ok && p.Owner() {
		return p.UserID
	}
	if sm.GetInt64(ctx, sessionSetupUntil) < time.Now().Unix() {
		return 0
	}
	return sm.GetInt64(ctx, sessionSetup)
}

// RequireRegistrant is operation middleware for making a passkey, which only a [Registrant]
// may do. A visitor by a share link without a code is refused as read-only, so the page they
// are on is not taken for signed out.
func RequireRegistrant(api huma.API, sm *scs.SessionManager) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if Registrant(ctx.Context(), sm) > 0 {
			next(ctx)
			return
		}
		if _, ok := FromContext(ctx.Context()); ok {
			huma.WriteErr(api, ctx, http.StatusForbidden, "a shared view is read-only")
			return
		}
		huma.WriteErr(api, ctx, http.StatusUnauthorized, "sign in, or take a setup code, first")
	}
}

// SignIn starts a session for the user, on a fresh token so a token from before the sign-in
// is worth nothing after it, and the whole lifetime. A visitor's link is set aside: the owner
// is the owner. The passkey used is kept, so removing it signs this device out.
func SignIn(ctx context.Context, sm *scs.SessionManager, userID int64, credentialID string) error {
	if err := sm.RenewToken(ctx); err != nil {
		return err
	}
	sm.Remove(ctx, sessionSetup)
	sm.Remove(ctx, sessionSetupUntil)
	sm.Remove(ctx, sessionShare)
	sm.Put(ctx, sessionUser, userID)
	sm.Put(ctx, sessionPasskey, credentialID)
	return nil
}

// splitStore keeps sessions in one table and reads them on the read pool: every request
// reads its session, so reads must not queue behind a write.
// errSignedOut is a session's sign-in that no longer holds: its user, or the passkey they
// signed in with, is gone.
var errSignedOut = errors.New("signed out")

// signedIn is the session's user, while the passkey they signed in with is still theirs:
// errSignedOut when it or they are gone, any other error when that cannot be read now. A
// dev-session's sign-in holds only while every origin is this machine.
func signedIn(ctx context.Context, q *db.Queries, id int64, cred string, local bool) (*db.User, error) {
	switch {
	case cred == devPasskey && local:
	case cred == "" || cred == devPasskey:
		return nil, errSignedOut
	default:
		n, err := q.CountUserPasskey(ctx, db.CountUserPasskeyParams{UserID: id, CredentialID: cred})
		if err != nil {
			return nil, fmt.Errorf("passkey: %w", err)
		}
		if n == 0 {
			return nil, errSignedOut
		}
	}
	u, err := q.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errSignedOut
	}
	return u, err
}

type splitStore struct {
	read, write scs.Store
}

func (s splitStore) Find(token string) ([]byte, bool, error) { return s.read.Find(token) }

func (s splitStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.write.Commit(token, b, expiry)
}

func (s splitStore) Delete(token string) error { return s.write.Delete(token) }

// NewSessionToken signs the user in without a browser and returns the cookie's value. The
// command that uses it refuses to run anywhere but a development machine.
func NewSessionToken(ctx context.Context, sm *scs.SessionManager, userID int64) (string, error) {
	ctx, err := sm.Load(ctx, "")
	if err != nil {
		return "", err
	}
	sm.Put(ctx, sessionUser, userID)
	sm.Put(ctx, sessionPasskey, devPasskey)
	token, _, err := sm.Commit(ctx)
	return token, err
}

// EnsureOwner returns the library's one user, creating it on first use.
func EnsureOwner(ctx context.Context, q *db.Queries, name string) (*db.User, error) {
	u, err := q.GetFirstUser(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return q.CreateUser(ctx, name)
	}
	return u, err
}

// Setup codes are eight letters and digits that cannot be misread for one another.
const codeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

// NewSetupCode makes a one-time code that lets a device make a passkey for the user on
// whichever name it is opened. Only its hash is kept.
func NewSetupCode(ctx context.Context, q *db.Queries, userID int64, ttl time.Duration) (string, time.Time, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", time.Time{}, err
	}
	code := make([]byte, 0, 9)
	for i, v := range b {
		if i == 4 {
			code = append(code, '-')
		}
		code = append(code, codeAlphabet[int(v)%len(codeAlphabet)])
	}
	expires := time.Now().UTC().Add(ttl)
	if err := q.DeleteExpiredSetupCodes(ctx, stamp(time.Now())); err != nil {
		return "", time.Time{}, err
	}
	if err := q.CreateSetupCode(ctx, db.CreateSetupCodeParams{CodeHash: hashCode(string(code)), UserID: userID, ExpiresAt: stamp(expires)}); err != nil {
		return "", time.Time{}, err
	}
	return string(code), expires, nil
}

func hashCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if strings.ContainsRune(codeAlphabet, r) {
			b.WriteRune(r)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func stamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
