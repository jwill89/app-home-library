package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/db"
)

type meOutput struct {
	Body struct {
		Role string       `json:"role" enum:"owner,visitor,none" doc:"The owner, a visitor by a share link, or no one"`
		Name string       `json:"name" doc:"The owner's name, for the owner and a visitor; empty for no one"`
		Sees catalog.Sees `json:"sees" doc:"Which of the library's pages have anything for the asker: all of them for the owner"`
	}
}

// me is who is asking, as get-me answers: ok says anyone is.
func me(p auth.Principal, ok bool) *meOutput {
	out := &meOutput{}
	out.Body.Role = "none"
	if ok {
		out.Body.Role, out.Body.Name, out.Body.Sees = "visitor", p.Name, p.Scope.Sees()
		if p.Owner() {
			out.Body.Role = "owner"
		}
	}
	return out
}

type passkey struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Site       string `json:"site" doc:"The name the passkey works on"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
}

type passkeyOutput struct{ Body passkey }

type addPasskeyInput struct {
	Body struct {
		Name       string          `json:"name,omitempty" maxLength:"60" doc:"What to call it, such as the device it lives on; \"Passkey\" when left out or blank."`
		Credential json.RawMessage `json:"credential" doc:"The browser's answer: the PublicKeyCredential's toJSON()."`
	}
}

type passkeysOutput struct {
	Body struct {
		Passkeys []passkey `json:"passkeys"`
	}
}

type setupCodeOutput struct {
	Body struct {
		Code      string `json:"code"`
		ExpiresAt string `json:"expiresAt"`
	}
}

func (h *handlers) registerAccount() {
	huma.Register(h.api, huma.Operation{
		OperationID: "get-me", Method: http.MethodGet, Path: "/me",
		Summary: "Who is asking: the owner, a visitor by a share link, or no one",
	}, func(ctx context.Context, _ *struct{}) (*meOutput, error) {
		return me(auth.FromContext(ctx)), nil
	})

	huma.Register(h.api, h.op("list-passkeys", http.MethodGet, "/passkeys", "The owner's passkeys on every name"),
		func(ctx context.Context, _ *struct{}) (*passkeysOutput, error) {
			p, _ := auth.FromContext(ctx)
			rows, err := db.New(h.DB.Read).ListPasskeys(ctx, p.UserID)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &passkeysOutput{}
			out.Body.Passkeys = []passkey{}
			for _, r := range rows {
				out.Body.Passkeys = append(out.Body.Passkeys, passkey{ID: r.ID, Name: r.Name, Site: r.RpID, CreatedAt: r.CreatedAt, LastUsedAt: r.LastUsedAt.String})
			}
			return out, nil
		})

	// Making a passkey is open to a device that has redeemed a setup code as well as to the
	// owner, so the first passkey on each name can be made.
	begin := h.op("begin-passkey", http.MethodPost, "/passkeys/begin", "Start making a passkey on this name")
	begin.Description = "For the owner, or a device that has redeemed a setup code (redeem-setup-code); a visitor by a share link is refused."
	begin.Middlewares = huma.Middlewares{auth.RequireRegistrant(h.api, h.Sessions)}
	begin.Errors = []int{http.StatusUnauthorized, http.StatusForbidden}
	huma.Register(h.api, begin, func(ctx context.Context, _ *struct{}) (*passkeyOptionsOutput, error) {
		opts, err := h.passkeys.BeginRegistration(ctx, requestOf(ctx).Site, auth.Registrant(ctx, h.Sessions))
		if err != nil {
			return nil, h.fail(err)
		}
		return &passkeyOptionsOutput{Body: PasskeyOptions{PublicKey: opts}}, nil
	})

	add := h.op("add-passkey", http.MethodPost, "/passkeys", "Keep the passkey the browser made")
	add.Description = "Finishes begin-passkey. A device that redeemed a setup code is signed in by it."
	add.Middlewares = begin.Middlewares
	add.DefaultStatus = http.StatusCreated
	add.Errors = []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict}
	huma.Register(h.api, add, func(ctx context.Context, in *addPasskeyInput) (*passkeyOutput, error) {
		row, err := h.passkeys.FinishRegistration(ctx, requestOf(ctx).Site, auth.Registrant(ctx, h.Sessions), in.Body.Name, in.Body.Credential)
		switch {
		case errors.Is(err, auth.ErrNoCeremony):
			return nil, huma.Error400BadRequest("No passkey is being made. Start again.")
		case errors.Is(err, auth.ErrNotAccepted):
			return nil, huma.Error400BadRequest("The passkey was not accepted. Try again.")
		case errors.Is(err, auth.ErrTaken):
			return nil, huma.Error409Conflict("That passkey is already added.")
		case err != nil:
			return nil, h.fail(err)
		}
		return &passkeyOutput{Body: passkey{ID: row.ID, Name: row.Name, Site: row.RpID, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt.String}}, nil
	})

	huma.Register(h.api, h.op("delete-passkey", http.MethodDelete, "/passkeys/{id}", "Remove a passkey"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			p, _ := auth.FromContext(ctx)
			n, err := db.New(h.DB.Write).DeletePasskey(ctx, db.DeletePasskeyParams{ID: in.ID, UserID: p.UserID})
			if err != nil {
				return nil, h.fail(err)
			}
			if n == 0 {
				return nil, huma.Error404NotFound("no such passkey")
			}
			return nil, nil
		})

	huma.Register(h.api, h.op("create-setup-code", http.MethodPost, "/setup-codes", "A one-time code that adds a passkey on another device or name"),
		func(ctx context.Context, _ *struct{}) (*setupCodeOutput, error) {
			p, _ := auth.FromContext(ctx)
			code, expires, err := auth.NewSetupCode(ctx, db.New(h.DB.Write), p.UserID, 15*time.Minute)
			if err != nil {
				return nil, h.fail(err)
			}
			out := &setupCodeOutput{}
			out.Body.Code, out.Body.ExpiresAt = code, expires.Format(time.RFC3339)
			return out, nil
		})
}
