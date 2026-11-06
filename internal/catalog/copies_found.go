package catalog

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"homelibrary/internal/lookup"
)

// citationLimit is far more than one record's citation: a RIS export of one article is a few
// kilobytes, its abstract the most of it.
const citationLimit = 1 << 20

// FillFromCitation fills a draft's blanks from a citation the owner exported from a library's
// database, as RIS, as a DOI's lookup does. The owner downloads it, signed in to the library:
// the app never signs in to a database itself.
func (s *Service) FillFromCitation(ris string, draft DocumentInput) (*DocumentFill, error) {
	if len(ris) > citationLimit {
		return nil, &FieldError{"ris", "that file is bigger than one citation: export just the one record"}
	}
	p := lookup.ParseRIS([]byte(ris))
	if p == nil {
		return nil, &FieldError{"ris", "there is no citation in that file. Export the record as RIS from the library's database, and choose that file."}
	}
	filled := fillDocumentBlanks(&draft, documentOf(p))
	return &DocumentFill{Document: draft, Filled: filled}, nil
}

// OpenCopy is where OpenAlex knows the document can be read without a subscription; nil when
// it knows none, or the document has no DOI to ask by.
func (s *Service) OpenCopy(ctx context.Context, id int64) (*lookup.OpenCopy, error) {
	d, err := s.read().GetDocument(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if d.Doi == "" {
		return nil, nil
	}
	c, err := s.Lookup.OpenCopyByDOI(ctx, d.Doi)
	if err != nil {
		return nil, fmt.Errorf("open copy of %s: %w", d.Doi, err)
	}
	return c, nil
}

// AddOpenCopy fetches the PDF of the document's open copy and adds it to its files. The link
// is OpenAlex's, never one sent from the browser.
func (s *Service) AddOpenCopy(ctx context.Context, id int64) (*Document, error) {
	c, err := s.OpenCopy(ctx, id)
	if err != nil {
		return nil, err
	}
	if c == nil || c.PDFURL == "" {
		return nil, &ConflictError{"No open copy's PDF is known for this document. Open the copy's page if there is one, or look in your library."}
	}
	unreached := &ConflictError{"The open copy could not be fetched from the server. Open it, download the PDF, and add it here."}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.PDFURL, nil)
	if err != nil {
		return nil, unreached
	}
	req.Header.Set("User-Agent", s.UserAgent)
	req.Header.Set("Accept", "application/pdf")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		s.Logger.Warn("open copy", "document", id, "url", c.PDFURL, "err", err)
		return nil, unreached
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		s.Logger.Warn("open copy", "document", id, "url", c.PDFURL, "status", resp.Status)
		return nil, unreached
	}
	// A repository sometimes answers with a page about the file rather than the file.
	br := bufio.NewReader(resp.Body)
	if head, _ := br.Peek(5); !bytes.Equal(head, []byte("%PDF-")) {
		return nil, &ConflictError{"The open copy's link gives a web page, not the PDF. Open it, download the PDF, and add it here."}
	}
	d, err := s.read().GetDocument(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.AddDocumentFile(ctx, id, d.Title+".pdf", br)
}
