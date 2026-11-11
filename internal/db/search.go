package db

import (
	"context"
	"fmt"
)

// The full-text index's statements are written here by hand, with the backup's VACUUM INTO the
// only SQL outside sqlc: its SQLite parser knows neither an FTS5 table's rowid nor that the
// table's own name is a column, which MATCH, snippet, and rank all need. A document's row in
// the index is its own id, so one document's entry is found without scanning the others'
// text.

// IndexDocumentParams is what the index holds of a document.
type IndexDocumentParams struct {
	DocumentID int64
	Title      string
	Authors    string
	Container  string
	Subjects   string
	Abstract   string
	Notes      string
	Body       string
}

// IndexDocument replaces a document's row in the full-text index.
func (q *Queries) IndexDocument(ctx context.Context, arg IndexDocumentParams) error {
	if err := q.ClearDocumentSearch(ctx, arg.DocumentID); err != nil {
		return err
	}
	_, err := q.db.ExecContext(ctx, `INSERT INTO document_search (rowid, title, authors, container, subjects, abstract, notes, body)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, arg.DocumentID, arg.Title, arg.Authors, arg.Container, arg.Subjects, arg.Abstract, arg.Notes, arg.Body)
	if err != nil {
		return fmt.Errorf("index document %d: %w", arg.DocumentID, err)
	}
	return nil
}

// ClearDocumentSearch removes a document's row from the full-text index.
func (q *Queries) ClearDocumentSearch(ctx context.Context, documentID int64) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM document_search WHERE rowid = ?`, documentID); err != nil {
		return fmt.Errorf("clear document %d from the index: %w", documentID, err)
	}
	return nil
}

// SearchHit is a document the full-text index matched, with the passage that matched, its
// words between char(2) and char(3).
type SearchHit struct {
	DocumentID int64
	Snippet    string
}

// SearchDocuments runs an FTS5 query, best matches first.
func (q *Queries) SearchDocuments(ctx context.Context, match string) ([]SearchHit, error) {
	rows, err := q.db.QueryContext(ctx, `SELECT rowid, snippet(document_search, -1, char(2), char(3), '…', 14)
FROM document_search WHERE document_search MATCH ? ORDER BY rank LIMIT 200`, match)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.DocumentID, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
