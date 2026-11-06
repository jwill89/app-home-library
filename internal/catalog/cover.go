package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"homelibrary/internal/covers"
	"homelibrary/internal/db"
	"homelibrary/internal/lookup"
)

// SourceOwner marks what the owner typed, photographed, or uploaded, a field left blank on
// purpose among them: nothing a source gives takes its place.
const SourceOwner = "owner"

// CoverFromURL fetches a cover from a link and makes it the edition's, named by the source
// the link belongs to, however it was chosen.
func (s *Service) CoverFromURL(ctx context.Context, id int64, url string) error {
	res, err := covers.Download(ctx, s.HTTP, s.UserAgent, url)
	if err != nil {
		return err
	}
	return s.setCover(ctx, id, res, sourceOfCover(url), false)
}

// CoverFromUpload makes an uploaded image the edition's cover.
func (s *Service) CoverFromUpload(ctx context.Context, id int64, r io.ReadSeeker) error {
	res, err := covers.Process(ctx, r)
	if err != nil {
		return err
	}
	return s.setCover(ctx, id, res, SourceOwner, false)
}

// RemoveCover leaves the edition without a cover.
func (s *Service) RemoveCover(ctx context.Context, id int64) error {
	return s.setCover(ctx, id, nil, "", false)
}

// coverIfNone fetches a cover a lookup found and keeps it only if the book still has none
// when it is stored: the owner may have photographed one while it downloaded.
func (s *Service) coverIfNone(ctx context.Context, id int64, url string) error {
	res, err := covers.Download(ctx, s.HTTP, s.UserAgent, url)
	if err != nil {
		return err
	}
	return s.setCover(ctx, id, res, sourceOfCover(url), true)
}

// setCover stores the new files first, then points the edition at them, then removes the
// old ones: a failure part way leaves files nothing names, never a cover with no files.
// ifNone leaves a cover the edition has, and removes the new files.
func (s *Service) setCover(ctx context.Context, id int64, res *covers.Result, source string, ifNone bool) error {
	name := ""
	if res != nil {
		var err error
		if name, err = s.Covers.Save(res); err != nil {
			return err
		}
	}
	var old string
	err := s.inTx(ctx, func(q *db.Queries) error {
		ed, err := q.GetEdition(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if ifNone && ed.Cover != "" {
			old = name // the new files go; the cover there stays
			return nil
		}
		old = ed.Cover
		sources := map[string]string{}
		_ = json.Unmarshal([]byte(ed.Sources), &sources) // written by this package
		if source == "" {
			delete(sources, "cover")
		} else {
			sources["cover"] = source
		}
		b, _ := json.Marshal(sources)
		return q.SetEditionCover(ctx, db.SetEditionCoverParams{Cover: name, Sources: string(b), ID: id})
	})
	if err != nil {
		if rmErr := s.Covers.Remove(name); rmErr != nil {
			s.Logger.Warn("cover files left behind", "cover", name, "err", rmErr)
		}
		return err
	}
	if err := s.Covers.Remove(old); err != nil {
		s.Logger.Warn("old cover files left behind", "cover", old, "err", err)
	}
	return nil
}

// sourceOfCover names the source a cover link belongs to, for the edition's field sources.
func sourceOfCover(url string) string {
	switch {
	case strings.Contains(url, "openlibrary.org"):
		return lookup.SourceOpenLibrary
	case strings.Contains(url, "google"):
		return lookup.SourceGoogle
	case strings.Contains(url, "hardcover.app"):
		return lookup.SourceHardcover
	}
	return "web"
}
