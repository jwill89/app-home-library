package api

import (
	"context"
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

type passkey struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Site       string `json:"site" doc:"The name the passkey works on"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt"`
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
		out := &meOutput{}
		out.Body.Role = "none"
		if p, ok := auth.FromContext(ctx); ok {
			out.Body.Role, out.Body.Name, out.Body.Sees = "visitor", p.Name, p.Scope.Sees()
			if p.Owner() {
				out.Body.Role = "owner"
			}
		}
		return out, nil
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
