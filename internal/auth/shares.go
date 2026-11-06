package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"

	"homelibrary/internal/catalog"
	"homelibrary/internal/db"
)

const sessionShare = "share_token" // the link's token hash

// ErrNoShare is a share link that is not there.
var ErrNoShare = errors.New("no such share link")

// ShareLink is a share link as Settings lists it; the link itself is shown only once.
type ShareLink struct {
	ID         int64         `json:"id"`
	Name       string        `json:"name"`
	ExpiresAt  string        `json:"expiresAt" doc:"When the link stops working; empty for never"`
	CreatedAt  string        `json:"createdAt"`
	LastUsedAt string        `json:"lastUsedAt" doc:"When it was last opened; empty for never"`
	Scope      catalog.Scope `json:"scope" doc:"What the link shows"`
}

// linkOf is a link as Settings lists it, with what it shows.
func linkOf(ctx context.Context, q *db.Queries, s *db.Share) (ShareLink, error) {
	scope, err := scopeOf(ctx, q, s)
	if err != nil {
		return ShareLink{}, err
	}
	return ShareLink{ID: s.ID, Name: s.Name, ExpiresAt: s.ExpiresAt, CreatedAt: s.CreatedAt, LastUsedAt: s.LastUsedAt, Scope: scope}, nil
}

// scopeOf is what a link shows: its choices of books and papers, and those it names.
func scopeOf(ctx context.Context, q *db.Queries, s *db.Share) (catalog.Scope, error) {
	sc := catalog.Scope{
		Books: s.Books, Wishlist: s.Wishlist == 1, Papers: s.Papers,
		Categories: []string{}, BookIDs: []int64{}, CourseIDs: []int64{}, PaperIDs: []int64{},
	}
	rows, err := q.ListShareScope(ctx, s.ID)
	if err != nil {
		return sc, fmt.Errorf("share scope: %w", err)
	}
	for _, r := range rows {
		switch r.Kind {
		case "category":
			sc.Categories = append(sc.Categories, r.Name)
		case "book":
			sc.BookIDs = append(sc.BookIDs, r.Ref)
		case "course":
			sc.CourseIDs = append(sc.CourseIDs, r.Ref)
		case "paper":
			sc.PaperIDs = append(sc.PaperIDs, r.Ref)
		}
	}
	return sc, nil
}

// writeScope keeps what a link shows in place of what it showed.
func writeScope(ctx context.Context, q *db.Queries, id int64, sc catalog.Scope) error {
	for _, empty := range []func(context.Context, int64) error{
		q.ClearShareCategories, q.ClearShareBooks, q.ClearShareCourses, q.ClearShareDocuments,
	} {
		if err := empty(ctx, id); err != nil {
			return fmt.Errorf("share scope: %w", err)
		}
	}
	for _, c := range sc.Categories {
		if err := q.AddShareCategory(ctx, db.AddShareCategoryParams{ShareID: id, Category: c}); err != nil {
			return fmt.Errorf("share category: %w", err)
		}
	}
	for _, b := range sc.BookIDs {
		if err := q.AddShareBook(ctx, db.AddShareBookParams{ShareID: id, EditionID: b}); err != nil {
			return fmt.Errorf("share book: %w", err)
		}
	}
	for _, c := range sc.CourseIDs {
		if err := q.AddShareCourse(ctx, db.AddShareCourseParams{ShareID: id, CourseID: c}); err != nil {
			return fmt.Errorf("share course: %w", err)
		}
	}
	for _, d := range sc.PaperIDs {
		if err := q.AddShareDocument(ctx, db.AddShareDocumentParams{ShareID: id, DocumentID: d}); err != nil {
			return fmt.Errorf("share paper: %w", err)
		}
	}
	return nil
}

func wishlistInt(sc catalog.Scope) int64 {
	if sc.Wishlist {
		return 1
	}
	return 0
}

// endStamp is a link's end as kept: empty for none.
func endStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return stamp(t)
}

// hashToken is what is kept of a share link's token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken is a secret for an address: random, and kept only as its hash.
func newToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// NewShare makes a share link that shows what the scope chooses, and returns it with its
// token, which is not kept and cannot be shown again. A zero expiry is a link with no end.
func NewShare(ctx context.Context, d *db.DB, name string, expires time.Time, sc catalog.Scope) (ShareLink, string, error) {
	if err := sc.Normalize(); err != nil {
		return ShareLink{}, "", err
	}
	token, err := newToken()
	if err != nil {
		return ShareLink{}, "", err
	}
	var link ShareLink
	err = d.InTx(ctx, func(q *db.Queries) error {
		s, err := q.CreateShare(ctx, db.CreateShareParams{
			Name: strings.TrimSpace(name), TokenHash: hashToken(token), ExpiresAt: endStamp(expires),
			Books: sc.Books, Wishlist: wishlistInt(sc), Papers: sc.Papers,
		})
		if err != nil {
			return fmt.Errorf("create share: %w", err)
		}
		if err := writeScope(ctx, q, s.ID, sc); err != nil {
			return err
		}
		link, err = linkOf(ctx, q, s)
		return err
	})
	if err != nil {
		return ShareLink{}, "", err
	}
	return link, token, nil
}

// UpdateShare renames a link, gives it a new end (or none), and changes what it shows. Its
// address stays the same, and its visitors see the change on their next request.
func UpdateShare(ctx context.Context, d *db.DB, id int64, name string, expires time.Time, sc catalog.Scope) (ShareLink, error) {
	if err := sc.Normalize(); err != nil {
		return ShareLink{}, err
	}
	var link ShareLink
	err := d.InTx(ctx, func(q *db.Queries) error {
		s, err := q.UpdateShare(ctx, db.UpdateShareParams{
			Name: strings.TrimSpace(name), ExpiresAt: endStamp(expires),
			Books: sc.Books, Wishlist: wishlistInt(sc), Papers: sc.Papers, ID: id,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoShare
		}
		if err != nil {
			return fmt.Errorf("update share: %w", err)
		}
		if err := writeScope(ctx, q, id, sc); err != nil {
			return err
		}
		link, err = linkOf(ctx, q, s)
		return err
	})
	return link, err
}

// Shares lists every share link, newest first, ended ones included until removed.
func Shares(ctx context.Context, q *db.Queries) ([]ShareLink, error) {
	rows, err := q.ListShares(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ShareLink, 0, len(rows))
	for _, s := range rows {
		link, err := linkOf(ctx, q, s)
		if err != nil {
			return nil, err
		}
		out = append(out, link)
	}
	return out, nil
}

// RevokeShare removes a share link; everyone browsing through it is turned away on their
// next request.
func RevokeShare(ctx context.Context, q *db.Queries, id int64) error {
	n, err := q.DeleteShare(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNoShare
	}
	return nil
}

// ShareLinks serves the share links' own address, /s/{token}.
type ShareLinks struct {
	Sessions *scs.SessionManager
	DB       *db.DB
	Logger   *slog.Logger
}

// Register puts the route on the mux.
func (s *ShareLinks) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /s/{token}", s.open)
}

// open starts a visitor's session for a link that is still running, and sends them to the
// library. The token leaves the address bar with the redirect. The owner, already signed in,
// stays the owner.
func (s *ShareLinks) open(w http.ResponseWriter, r *http.Request) {
	if p, ok := FromContext(r.Context()); ok && p.Owner() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	share, err := db.New(s.DB.Read).ShareByToken(r.Context(), db.ShareByTokenParams{
		TokenHash: hashToken(r.PathValue("token")), Now: stamp(time.Now()),
	})
	if errors.Is(err, sql.ErrNoRows) {
		http.Redirect(w, r, "/link-ended", http.StatusSeeOther)
		return
	}
	if err != nil {
		s.Logger.Error("share link", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	if err := s.Sessions.RenewToken(r.Context()); err != nil {
		s.Logger.Error("share link session", "err", err)
		http.Error(w, "something went wrong", http.StatusInternalServerError)
		return
	}
	s.Sessions.Remove(r.Context(), sessionSetup)
	s.Sessions.Put(r.Context(), sessionShare, share.TokenHash)
	if err := db.New(s.DB.Write).TouchShare(context.WithoutCancel(r.Context()), share.ID); err != nil {
		s.Logger.Warn("share link last used", "err", err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
