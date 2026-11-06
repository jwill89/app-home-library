package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"homelibrary/internal/auth"
	"homelibrary/internal/db"
)

type calendarLinkOutput struct {
	Body auth.CalendarLink
}

type newCalendarLinkOutput struct {
	Body struct {
		Token string `json:"token" doc:"The link's secret part, shown this once: the link is /calendar/ and this, then .ics"`
	}
}

func (h *handlers) registerCalendar() {
	huma.Register(h.api, h.op("get-calendar-link", http.MethodGet, "/calendar-link", "Whether there is a calendar link for the loans' due dates"),
		func(ctx context.Context, _ *struct{}) (*calendarLinkOutput, error) {
			c, err := auth.GetCalendarLink(ctx, db.New(h.DB.Read))
			if err != nil {
				return nil, h.fail(err)
			}
			return &calendarLinkOutput{Body: c}, nil
		})

	create := h.op("create-calendar-link", http.MethodPost, "/calendar-link", "Make the calendar link, replacing the one before")
	create.DefaultStatus = http.StatusCreated
	huma.Register(h.api, create, func(ctx context.Context, _ *struct{}) (*newCalendarLinkOutput, error) {
		token, err := auth.NewCalendarLink(ctx, db.New(h.DB.Write))
		if err != nil {
			return nil, h.fail(err)
		}
		out := &newCalendarLinkOutput{}
		out.Body.Token = token
		return out, nil
	})

	huma.Register(h.api, h.op("delete-calendar-link", http.MethodDelete, "/calendar-link", "Turn the calendar link off"),
		func(ctx context.Context, _ *struct{}) (*struct{}, error) {
			return nil, h.fail0(auth.RevokeCalendarLink(ctx, db.New(h.DB.Write)))
		})
}
