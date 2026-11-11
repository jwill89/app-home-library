package auth

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"homelibrary/internal/db"
)

// Passkeys runs the passkey ceremonies on the session the request's context carries. Each
// ceremony has two steps; its challenge waits in the session between them, for ten minutes at
// most, and answers once. site is the configured origin the request came on, scheme and host.
type Passkeys struct {
	Sessions *scs.SessionManager
	DB       *db.DB
	Logger   *slog.Logger
}

// The ways a ceremony is refused, which the API answers each in its own words.
var (
	// ErrNoCeremony is a ceremony's second step with no first step waiting in the session.
	ErrNoCeremony = errors.New("no passkey ceremony under way")
	// ErrRefused is an answer to a sign-in that signs no one in.
	ErrRefused = errors.New("passkey refused")
	// ErrNotAccepted is a new passkey that does not check out against its challenge.
	ErrNotAccepted = errors.New("passkey not accepted")
	// ErrTaken is a new passkey whose credential is already kept.
	ErrTaken = errors.New("passkey already added")
	// ErrWrongCode is a setup code that is wrong, used, or expired.
	ErrWrongCode = errors.New("setup code wrong, used, or expired")
)

const (
	keyRegister = "webauthn_register"
	keyLogin    = "webauthn_login"
)

// relyingParty is the WebAuthn configuration for site: its host is the relying party, and it
// is the one origin a browser's answer may come from. The challenge holds as long as the
// session of someone part way in.
func relyingParty(site string) (*webauthn.WebAuthn, string, error) {
	u, err := url.Parse(site)
	if err != nil || u.Hostname() == "" {
		return nil, "", fmt.Errorf("no site for WebAuthn (%q)", site)
	}
	timeout := webauthn.TimeoutConfig{Enforce: true, Timeout: unfinished, TimeoutUVD: unfinished}
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Home Library",
		RPID:          u.Hostname(),
		RPOrigins:     []string{site},
		Timeouts:      webauthn.TimeoutsConfig{Login: timeout, Registration: timeout},
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

func (p *Passkeys) stash(ctx context.Context, key string, data *webauthn.SessionData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	p.Sessions.Put(ctx, key, string(b))
	return nil
}

func (p *Passkeys) unstash(ctx context.Context, key string) (*webauthn.SessionData, bool) {
	raw := p.Sessions.PopString(ctx, key)
	if raw == "" {
		return nil, false
	}
	var data webauthn.SessionData
	if json.Unmarshal([]byte(raw), &data) != nil {
		return nil, false
	}
	return &data, true
}

// Redeem spends a setup code: the session moves to a fresh token and may make one passkey for
// the code's user, within ten minutes. The leave ends by its own clock, which nothing renews;
// a session that was no one's lasts only as long, and the owner's or a visitor's keeps its own.
func (p *Passkeys) Redeem(ctx context.Context, code string) error {
	id, err := db.New(p.DB.Write).TakeSetupCode(ctx, db.TakeSetupCodeParams{CodeHash: hashCode(code), ExpiresAt: stamp(time.Now())})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWrongCode
	}
	if err != nil {
		return err
	}
	if err := p.Sessions.RenewToken(ctx); err != nil {
		return err
	}
	until := time.Now().Add(unfinished)
	p.Sessions.Put(ctx, sessionSetup, id)
	p.Sessions.Put(ctx, sessionSetupUntil, until.Unix())
	if _, ok := FromContext(ctx); !ok {
		p.Sessions.SetDeadline(ctx, until)
	}
	return nil
}

// BeginRegistration starts making a passkey for the user on site, and answers the options the
// browser makes it from: a discoverable key, so signing in asks for nothing but the
// fingerprint, and none the authenticator already holds for this name.
func (p *Passkeys) BeginRegistration(ctx context.Context, site string, userID int64) (json.RawMessage, error) {
	wa, rpID, err := relyingParty(site)
	if err != nil {
		return nil, err
	}
	o, err := p.load(ctx, userID, rpID)
	if err != nil {
		return nil, err
	}
	exclude := make([]protocol.CredentialDescriptor, 0, len(o.creds))
	for i := range o.creds {
		exclude = append(exclude, o.creds[i].Descriptor())
	}
	creation, data, err := wa.BeginRegistration(o,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationPreferred,
		}),
		webauthn.WithExclusions(exclude),
	)
	if err != nil {
		return nil, err
	}
	if err := p.stash(ctx, keyRegister, data); err != nil {
		return nil, err
	}
	return json.Marshal(creation.Response)
}

// FinishRegistration keeps the passkey the browser made, under name, or "Passkey" when the
// name is blank. A device holding a redeemed setup code is signed in by it.
func (p *Passkeys) FinishRegistration(ctx context.Context, site string, userID int64, name string, credential []byte) (*db.CreatePasskeyRow, error) {
	data, ok := p.unstash(ctx, keyRegister)
	if !ok {
		return nil, ErrNoCeremony
	}
	wa, rpID, err := relyingParty(site)
	if err != nil {
		return nil, err
	}
	o, err := p.load(ctx, userID, rpID)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(credential)
	if err != nil {
		p.Logger.Warn("passkey not accepted", "err", err)
		return nil, ErrNotAccepted
	}
	cred, err := wa.CreateCredential(o, *data, parsed)
	if err != nil {
		p.Logger.Warn("passkey not accepted", "err", err)
		return nil, ErrNotAccepted
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	if name = strings.TrimSpace(name); name == "" {
		name = "Passkey"
	}
	credentialID := base64.RawURLEncoding.EncodeToString(cred.ID)
	row, err := db.New(p.DB.Write).CreatePasskey(ctx, db.CreatePasskeyParams{
		UserID: userID, RpID: rpID, CredentialID: credentialID, Credential: string(raw), Name: name,
	})
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return nil, ErrTaken
	}
	if err != nil {
		return nil, err
	}
	if pr, ok := FromContext(ctx); !ok || !pr.Owner() {
		if err := SignIn(ctx, p.Sessions, userID, credentialID); err != nil {
			return nil, err
		}
	}
	return row, nil
}

// BeginSignIn starts a sign-in on site, and answers the options the browser offers one of its
// passkeys from. Someone with no session yet keeps this one only while they sign in.
func (p *Passkeys) BeginSignIn(ctx context.Context, site string) (json.RawMessage, error) {
	wa, _, err := relyingParty(site)
	if err != nil {
		return nil, err
	}
	assertion, data, err := wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationPreferred))
	if err != nil {
		return nil, err
	}
	if err := p.stash(ctx, keyLogin, data); err != nil {
		return nil, err
	}
	if _, ok := FromContext(ctx); !ok {
		p.Sessions.SetDeadline(ctx, time.Now().Add(unfinished))
	}
	return json.Marshal(assertion.Response)
}

// FinishSignIn checks the browser's answer to the sign-in under way and signs in the user whose
// passkey made it.
func (p *Passkeys) FinishSignIn(ctx context.Context, site string, credential []byte) (*db.User, error) {
	data, ok := p.unstash(ctx, keyLogin)
	if !ok {
		return nil, ErrNoCeremony
	}
	wa, rpID, err := relyingParty(site)
	if err != nil {
		return nil, err
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(credential)
	if err != nil {
		p.Logger.Warn("passkey sign-in refused", "err", err)
		return nil, ErrRefused
	}
	// The browser names the user by its handle; the user's passkeys on this name are what the
	// signature is checked against.
	var who *owner
	_, cred, err := wa.ValidatePasskeyLogin(func(_, handle []byte) (webauthn.User, error) {
		id, ok := handleToID(handle)
		if !ok {
			return nil, errors.New("unknown user handle")
		}
		o, err := p.load(ctx, id, rpID)
		who = o
		return o, err
	}, *data, parsed)
	if err != nil || who == nil {
		p.Logger.Warn("passkey sign-in refused", "err", err)
		return nil, ErrRefused
	}
	// The assertion is spent: its counter is saved even if the client leaves now.
	credentialID := base64.RawURLEncoding.EncodeToString(cred.ID)
	if raw, err := json.Marshal(cred); err == nil {
		if err := db.New(p.DB.Write).UsePasskey(context.WithoutCancel(ctx), db.UsePasskeyParams{Credential: string(raw), CredentialID: credentialID}); err != nil {
			p.Logger.Error("save passkey counter", "err", err)
		}
	}
	if err := SignIn(ctx, p.Sessions, who.user.ID, credentialID); err != nil {
		return nil, err
	}
	return who.user, nil
}
