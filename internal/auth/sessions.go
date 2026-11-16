package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"homelibrary/internal/db"
)

// sessionStore keeps sessions in the sessions table, each under its token's hash (the session
// manager hashes it first) with its expiry as a timestamp. A session is read on the read pool:
// every request reads its session, so reads must not queue behind a write.
type sessionStore struct {
	read, write *sql.DB
}

func (s sessionStore) Find(token string) ([]byte, bool, error) {
	return s.FindCtx(context.Background(), token)
}

func (s sessionStore) Commit(token string, b []byte, expiry time.Time) error {
	return s.CommitCtx(context.Background(), token, b, expiry)
}

func (s sessionStore) Delete(token string) error {
	return s.DeleteCtx(context.Background(), token)
}

// FindCtx is a running session's data, and false for one ended or never made.
func (s sessionStore) FindCtx(ctx context.Context, token string) ([]byte, bool, error) {
	b, err := db.New(s.read).FindSession(ctx, token)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("find session: %w", err)
	}
	return b, true, nil
}

// CommitCtx keeps a session's data until expiry, to the second.
func (s sessionStore) CommitCtx(ctx context.Context, token string, b []byte, expiry time.Time) error {
	err := db.New(s.write).SaveSession(ctx, db.SaveSessionParams{Token: token, Data: b, Expiry: stamp(expiry)})
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

func (s sessionStore) DeleteCtx(ctx context.Context, token string) error {
	if err := db.New(s.write).DeleteSession(ctx, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// RunSessionPruner lets the ended sessions go every hour until ctx ends. A session that has
// ended is never found, so this only gives the room back.
func RunSessionPruner(ctx context.Context, write *sql.DB, logger *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := db.New(write).DeleteExpiredSessions(ctx); err != nil && ctx.Err() == nil {
				logger.Error("prune sessions", "err", err)
			}
		}
	}
}
