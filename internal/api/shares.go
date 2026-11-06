package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/catalog"
	"homelibrary/internal/db"
)

type sharesOutput struct {
	Body struct {
		Shares []auth.ShareLink `json:"shares"`
	}
}

// shareInput is a link as the owner names it, ends it, and chooses what it shows.
type shareInput struct {
	Name      string        `json:"name" minLength:"1" maxLength:"80" doc:"Who or what the link is for"`
	ExpiresAt string        `json:"expiresAt" maxLength:"40" doc:"When the link stops working, RFC 3339; empty for never"`
	Scope     catalog.Scope `json:"scope"`
}

// end is when a link stops working: zero for never, and never a time already past.
func (in shareInput) end() (time.Time, error) {
	if in.ExpiresAt == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, in.ExpiresAt)
	if err != nil || !t.After(time.Now()) {
		return time.Time{}, huma.Error422UnprocessableEntity("The end date has to be in the future.",
			&huma.ErrorDetail{Location: "body.expiresAt", Message: "pick a day after today, or none"})
	}
	return t, nil
}

type shareOutput struct {
	Body auth.ShareLink
}

type newShareOutput struct {
	Body struct {
		Share auth.ShareLink `json:"share"`
		Token string         `json:"token" doc:"The link's secret part, shown this once: the link is /s/ and this"`
	}
}

func (h *handlers) registerShares() {
	huma.Register(h.api, h.op("list-shares", http.MethodGet, "/shares", "The read-only share links, newest first"),
		func(ctx context.Context, _ *struct{}) (*sharesOutput, error) {
			s, err := auth.Shares(ctx, db.New(h.DB.Read))
			if err != nil {
				return nil, h.fail(err)
			}
			out := &sharesOutput{}
			out.Body.Shares = s
			return out, nil
		})

	create := h.op("create-share", http.MethodPost, "/shares", "Make a read-only share link")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, in *struct{ Body shareInput }) (*newShareOutput, error) {
		expires, err := in.Body.end()
		if err != nil {
			return nil, err
		}
		s, token, err := auth.NewShare(ctx, h.DB, in.Body.Name, expires, in.Body.Scope)
		if err != nil {
			return nil, h.fail(err)
		}
		out := &newShareOutput{}
		out.Body.Share, out.Body.Token = s, token
		return out, nil
	})

	huma.Register(h.api, h.op("update-share", http.MethodPut, "/shares/{id}", "Rename a share link, change its end, or change what it shows; its address stays"),
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body shareInput
		}) (*shareOutput, error) {
			expires, err := in.Body.end()
			if err != nil {
				return nil, err
			}
			s, err := auth.UpdateShare(ctx, h.DB, in.ID, in.Body.Name, expires, in.Body.Scope)
			if errors.Is(err, auth.ErrNoShare) {
				return nil, huma.Error404NotFound("no such share link")
			}
			if err != nil {
				return nil, h.fail(err)
			}
			return &shareOutput{Body: s}, nil
		})

	huma.Register(h.api, h.op("delete-share", http.MethodDelete, "/shares/{id}", "Revoke a share link at once"),
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			err := auth.RevokeShare(ctx, db.New(h.DB.Write), in.ID)
			if errors.Is(err, auth.ErrNoShare) {
				return nil, huma.Error404NotFound("no such share link")
			}
			if err != nil {
				return nil, h.fail(err)
			}
			return nil, nil
		})
}
