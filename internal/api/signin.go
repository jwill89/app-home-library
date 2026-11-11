package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
)

// Signing in and out are actions, not resources, so each is a POST under /auth. They run on
// the session the server's session layer loaded into the request's context: signing in moves
// it to a new token and records the owner (auth.SignIn), and the layer then writes the cookie.

// limited says what the server's sign-in limit, counted by address across every operation
// here but signing out, does to a caller who goes past it.
const limited = "Each address has ten quick tries at signing in, then one every three seconds (429, with Retry-After)."

// PasskeyOptions is a ceremony's first step: what the browser needs to make or offer a
// passkey.
type PasskeyOptions struct {
	PublicKey json.RawMessage `json:"publicKey" doc:"The WebAuthn options, as PublicKeyCredential.parseCreationOptionsFromJSON (making a passkey) or parseRequestOptionsFromJSON (signing in) takes them."`
}

type passkeyOptionsOutput struct{ Body PasskeyOptions }

type passkeySignInInput struct {
	Body struct {
		Credential json.RawMessage `json:"credential" doc:"The browser's answer: the PublicKeyCredential's toJSON()."`
	}
}

type setupCodeInput struct {
	Body struct {
		Code string `json:"code" minLength:"1" maxLength:"32" pattern:"[\\p{L}\\p{N}\\p{P}\\p{S}]" patternDescription:"more than spaces" doc:"The setup code, as shown where it was made; case, spaces and the hyphen don't matter."`
	}
}

func (h *handlers) registerSignIn() {
	huma.Register(h.api, huma.Operation{
		OperationID: "begin-passkey-sign-in", Method: http.MethodPost, Path: "/auth/passkey/begin",
		Summary:     "Start signing in with a passkey",
		Description: "Answers the options the browser offers a passkey made on this name from. " + limited,
		Errors:      []int{http.StatusTooManyRequests},
	}, h.beginPasskeySignIn)
	huma.Register(h.api, huma.Operation{
		OperationID: "finish-passkey-sign-in", Method: http.MethodPost, Path: "/auth/passkey/finish",
		Summary:     "Sign in with the passkey the browser offered",
		Description: "Answers who is now signed in, as get-me does. " + limited,
		Errors:      []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusTooManyRequests},
	}, h.finishPasskeySignIn)
	huma.Register(h.api, huma.Operation{
		OperationID: "redeem-setup-code", Method: http.MethodPost, Path: "/auth/setup",
		Summary: "Take a setup code, so this device may make a passkey",
		Description: "Spends the code: for the next ten minutes this device may make one passkey on this name " +
			"(begin-passkey, then add-passkey), which signs it in. " + limited,
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusTooManyRequests},
	}, h.redeemSetupCode)
	huma.Register(h.api, huma.Operation{
		OperationID: "sign-out", Method: http.MethodPost, Path: "/auth/logout",
		Summary: "Sign out", DefaultStatus: http.StatusNoContent,
	}, h.signOut)
}

func (h *handlers) beginPasskeySignIn(ctx context.Context, _ *struct{}) (*passkeyOptionsOutput, error) {
	opts, err := h.passkeys.BeginSignIn(ctx, requestOf(ctx).Site)
	if err != nil {
		return nil, h.fail(err)
	}
	return &passkeyOptionsOutput{Body: PasskeyOptions{PublicKey: opts}}, nil
}

func (h *handlers) finishPasskeySignIn(ctx context.Context, in *passkeySignInInput) (*meOutput, error) {
	u, err := h.passkeys.FinishSignIn(ctx, requestOf(ctx).Site, in.Body.Credential)
	switch {
	case errors.Is(err, auth.ErrNoCeremony):
		return nil, huma.Error400BadRequest("No sign-in is under way. Start again.")
	case errors.Is(err, auth.ErrRefused):
		return nil, huma.Error401Unauthorized("That passkey did not sign you in. Use one made on this address, or set one up with a code.")
	case err != nil:
		return nil, h.fail(err)
	}
	// The principal in the context is who asked before signing in; the answer is who is now.
	return me(auth.Principal{UserID: u.ID, Name: u.Name}, true), nil
}

func (h *handlers) redeemSetupCode(ctx context.Context, in *setupCodeInput) (*struct{}, error) {
	err := h.passkeys.Redeem(ctx, in.Body.Code)
	if errors.Is(err, auth.ErrWrongCode) {
		return nil, huma.Error401Unauthorized("That code is wrong, used, or expired. Make a new one.")
	}
	if err != nil {
		return nil, h.fail(err)
	}
	return nil, nil
}

func (h *handlers) signOut(ctx context.Context, _ *struct{}) (*struct{}, error) {
	// Signed out either way: a session the store couldn't delete has already lost its cookie,
	// and expires on its own.
	if err := h.Sessions.Destroy(ctx); err != nil {
		h.Logger.Error("end a session", "err", err)
	}
	return nil, nil
}
