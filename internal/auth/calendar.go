package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"homelibrary/internal/db"
)

// CalendarLink is the calendar link as the Loans page shows it; the link itself is shown only
// once, when made.
type CalendarLink struct {
	Made       bool   `json:"made"`
	CreatedAt  string `json:"createdAt"`
	LastUsedAt string `json:"lastUsedAt" doc:"When a calendar last fetched it; empty for never"`
}

// GetCalendarLink says whether there is a calendar link, and when it was made and last used.
func GetCalendarLink(ctx context.Context, q *db.Queries) (CalendarLink, error) {
	c, err := q.GetCalendarLink(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return CalendarLink{}, nil
	}
	if err != nil {
		return CalendarLink{}, err
	}
	return CalendarLink{Made: true, CreatedAt: c.CreatedAt, LastUsedAt: c.LastUsedAt}, nil
}

// NewCalendarLink makes the calendar link, replacing any before it, and returns its token,
// which is not kept and cannot be shown again.
func NewCalendarLink(ctx context.Context, q *db.Queries) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if err := q.DeleteCalendarLinks(ctx); err != nil {
		return "", fmt.Errorf("replace calendar link: %w", err)
	}
	if _, err := q.CreateCalendarLink(ctx, hashToken(token)); err != nil {
		return "", fmt.Errorf("calendar link: %w", err)
	}
	return token, nil
}

// RevokeCalendarLink turns the calendar link off: a calendar subscribed to it gets nothing more.
func RevokeCalendarLink(ctx context.Context, q *db.Queries) error {
	return q.DeleteCalendarLinks(ctx)
}

// CalendarLinkFor reports whether a token is the calendar link's, and marks it used.
func CalendarLinkFor(ctx context.Context, write *db.Queries, token string) (bool, error) {
	c, err := write.CalendarLinkByToken(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, write.TouchCalendarLink(ctx, c.ID)
}
