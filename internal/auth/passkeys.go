package auth

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"homelibrary/internal/db"
)

// Passkeys serves the sign-in ceremonies under /auth.
type Passkeys struct {
	Sessions *scs.SessionManager
	DB       *db.DB
	// Origin maps a request's Host to the configured origin; a name not configured is refused.
	Origin func(host string) (string, bool)
	Logger *slog.Logger
}

// Register puts the routes on the mux.
func (p *Passkeys) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/setup", p.redeem)
	mux.HandleFunc("POST /auth/passkey/register/begin", p.registerBegin)
	mux.HandleFunc("POST /auth/passkey/register/finish", p.registerFinish)
	mux.HandleFunc("POST /auth/passkey/login/begin", p.loginBegin)
	mux.HandleFunc("POST /auth/passkey/login/finish", p.loginFinish)
	mux.HandleFunc("POST /auth/logout", p.logout)
}

const (
	keyRegister = "webauthn_register"
	keyLogin    = "webauthn_login"
)

// relyingParty is the WebAuthn configuration for the name the request came on.
func (p *Passkeys) relyingParty(r *http.Request) (*webauthn.WebAuthn, string, error) {
	origin, ok := p.Origin(r.Host)
	if !ok {
		return nil, "", errors.New("not a name this site answers on")
	}
	u, err := url.Parse(origin)
	if err != nil {
		return nil, "", err
	}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Home Library",
		RPID:          u.Hostname(),
		RPOrigins:     []string{origin},
	})
	return wa, u.Hostname(), err
}

// owner adapts the user and its passkeys on one name to go-webauthn. The user handle is the
// id's eight bytes: stable, and nothing personal.
type owner struct {
	user  *db.User
	creds []webauthn.Credential
}

func (o *owner) WebAuthnID() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(o.user.ID)) //nolint:gosec // a positive row id, read back by handleToID
	return b
}
func (o *owner) WebAuthnName() string                       { return o.user.Name }
func (o *owner) WebAuthnDisplayName() string                { return o.user.Name }
func (o *owner) WebAuthnCredentials() []webauthn.Credential { return o.creds }

func handleToID(h []byte) (int64, bool) {
	if len(h) != 8 {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(h)), true //nolint:gosec // the reverse of WebAuthnID
}

func (p *Passkeys) load(ctx context.Context, userID int64, rpID string) (*owner, error) {
	q := db.New(p.DB.Read)
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	raw, err := q.ListPasskeyCredentials(ctx, db.ListPasskeyCredentialsParams{UserID: userID, RpID: rpID})
	if err != nil {
		return nil, err
	}
	o := &owner{user: u}
	for _, j := range raw {
		var c webauthn.Credential
		if json.Unmarshal([]byte(j), &c) == nil {
			o.creds = append(o.creds, c)
		}
	}
	return o, nil
}

func (p *Passkeys) stash(r *http.Request, key string, data *webauthn.SessionData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	p.Sessions.Put(r.Context(), key, string(b))
	return nil
}

func (p *Passkeys) unstash(r *http.Request, key string) (*webauthn.SessionData, bool) {
	raw := p.Sessions.PopString(r.Context(), key)
	if raw == "" {
		return nil, false
	}
	var data webauthn.SessionData
	if json.Unmarshal([]byte(raw), &data) != nil {
		return nil, false
	}
	return &data, true
}

// registrant is who may make a passkey now: the signed-in owner, or a device that has just
// redeemed a setup code.
func (p *Passkeys) registrant(r *http.Request) int64 {
	if pr, ok := FromContext(r.Context()); ok && pr.Owner() {
		return pr.UserID
	}
	return p.Sessions.GetInt64(r.Context(), sessionSetup)
}

func (p *Passkeys) redeem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil || strings.TrimSpace(body.Code) == "" {
		problem(w, http.StatusBadRequest, "Type the setup code.")
		return
	}
	id, err := db.New(p.DB.Write).TakeSetupCode(r.Context(), db.TakeSetupCodeParams{CodeHash: hashCode(body.Code), ExpiresAt: stamp(time.Now())})
	if err != nil {
		problem(w, http.StatusUnauthorized, "That code is wrong, used, or expired. Make a new one.")
		return
	}
	if err := p.Sessions.RenewToken(r.Context()); err != nil {
		p.fail(w, "session", err)
		return
	}
	p.Sessions.Put(r.Context(), sessionSetup, id)
	p.Sessions.SetDeadline(r.Context(), time.Now().Add(unfinished))
	writeJSON(w, map[string]bool{"ok": true})
}

func (p *Passkeys) registerBegin(w http.ResponseWriter, r *http.Request) {
	userID := p.registrant(r)
	if userID == 0 {
		problem(w, http.StatusUnauthorized, "Sign in, or redeem a setup code, first.")
		return
	}
	wa, rpID, err := p.relyingParty(r)
	if err != nil {
		problem(w, http.StatusMisdirectedRequest, err.Error())
		return
	}
	o, err := p.load(r.Context(), userID, rpID)
	if err != nil {
		p.fail(w, "load passkeys", err)
		return
	}
	exclude := make([]protocol.CredentialDescriptor, 0, len(o.creds))
	for i := range o.creds {
		exclude = append(exclude, o.creds[i].Descriptor())
	}
	// A discoverable key, so signing in asks for nothing but the fingerprint.
	creation, data, err := wa.BeginRegistration(o,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithExclusions(exclude),
	)
	if err != nil {
		p.fail(w, "begin registration", err)
		return
	}
	if err := p.stash(r, keyRegister, data); err != nil {
		p.fail(w, "session", err)
		return
	}
	writeJSON(w, creation)
}

func (p *Passkeys) registerFinish(w http.ResponseWriter, r *http.Request) {
	userID := p.registrant(r)
	data, ok := p.unstash(r, keyRegister)
	if userID == 0 || !ok {
		problem(w, http.StatusBadRequest, "No passkey is being made. Start again.")
		return
	}
	wa, rpID, err := p.relyingParty(r)
	if err != nil {
		problem(w, http.StatusMisdirectedRequest, err.Error())
		return
	}
	o, err := p.load(r.Context(), userID, rpID)
	if err != nil {
		p.fail(w, "load passkeys", err)
		return
	}
	cred, err := wa.FinishRegistration(o, *data, r)
	if err != nil {
		p.Logger.Warn("passkey registration refused", "err", err)
		problem(w, http.StatusBadRequest, "The passkey was not accepted. Try again.")
		return
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		p.fail(w, "encode passkey", err)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = "Passkey"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	if _, err := db.New(p.DB.Write).CreatePasskey(r.Context(), db.CreatePasskeyParams{
		UserID: userID, RpID: rpID, CredentialID: base64.RawURLEncoding.EncodeToString(cred.ID), Credential: string(raw), Name: name,
	}); err != nil {
		p.fail(w, "save passkey", err)
		return
	}
	if pr, ok := FromContext(r.Context()); !ok || !pr.Owner() {
		if err := SignIn(p.Sessions, r, userID, base64.RawURLEncoding.EncodeToString(cred.ID)); err != nil {
			p.fail(w, "session", err)
			return
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (p *Passkeys) loginBegin(w http.ResponseWriter, r *http.Request) {
	wa, _, err := p.relyingParty(r)
	if err != nil {
		problem(w, http.StatusMisdirectedRequest, err.Error())
		return
	}
	assertion, data, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		p.fail(w, "begin login", err)
		return
	}
	if err := p.stash(r, keyLogin, data); err != nil {
		p.fail(w, "session", err)
		return
	}
	// Someone with no session yet keeps this one only while they sign in.
	if _, ok := FromContext(r.Context()); !ok {
		p.Sessions.SetDeadline(r.Context(), time.Now().Add(unfinished))
	}
	writeJSON(w, assertion)
}

func (p *Passkeys) loginFinish(w http.ResponseWriter, r *http.Request) {
	data, ok := p.unstash(r, keyLogin)
	if !ok {
		problem(w, http.StatusBadRequest, "No sign-in is under way. Start again.")
		return
	}
	wa, rpID, err := p.relyingParty(r)
	if err != nil {
		problem(w, http.StatusMisdirectedRequest, err.Error())
		return
	}
	var who *owner
	user, cred, err := wa.FinishPasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		id, ok := handleToID(handle)
		if !ok {
			return nil, errors.New("unknown user handle")
		}
		o, err := p.load(r.Context(), id, rpID)
		who = o
		return o, err
	}, *data, r)
	if err != nil || user == nil || who == nil {
		p.Logger.Warn("passkey sign-in refused", "err", err)
		problem(w, http.StatusUnauthorized, "That passkey did not sign you in. Use one made on this address, or set one up with a code.")
		return
	}
	// The assertion is spent: its counter is saved even if the client leaves now.
	ctx := context.WithoutCancel(r.Context())
	if raw, err := json.Marshal(cred); err == nil {
		if err := db.New(p.DB.Write).UsePasskey(ctx, db.UsePasskeyParams{Credential: string(raw), CredentialID: base64.RawURLEncoding.EncodeToString(cred.ID)}); err != nil {
			p.Logger.Error("save passkey counter", "err", err)
		}
	}
	if err := SignIn(p.Sessions, r, who.user.ID, base64.RawURLEncoding.EncodeToString(cred.ID)); err != nil {
		p.fail(w, "session", err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (p *Passkeys) logout(w http.ResponseWriter, r *http.Request) {
	if err := p.Sessions.Destroy(r.Context()); err != nil {
		p.fail(w, "session", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (p *Passkeys) fail(w http.ResponseWriter, what string, err error) {
	p.Logger.Error("auth", "step", what, "err", err)
	problem(w, http.StatusInternalServerError, "Something went wrong. Try again.")
}
