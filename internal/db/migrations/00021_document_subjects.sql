-- +goose Up

-- Documents take subjects, as books do, filled from a DOI by OpenAlex and Crossref or typed
-- (2026-11-05), and the search finds them. An FTS5 table takes no new column, so the index is
-- made again and filled from the documents and their files, as the catalog indexes one.
ALTER TABLE documents ADD COLUMN subjects TEXT NOT NULL DEFAULT '[]';

DROP TABLE document_search;

CREATE VIRTUAL TABLE document_search USING fts5 (
    document_id UNINDEXED, title, authors, container, subjects, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

INSERT INTO document_search (document_id, title, authors, container, subjects, notes, body)
SELECT
    CAST(d.id AS TEXT), d.title,
    COALESCE((SELECT group_concat(a.value, '; ') FROM json_each(d.authors) AS a), ''),
    d.container, '', d.notes,
    COALESCE((
        SELECT group_concat(f.text, char(10))
        FROM (SELECT text FROM document_files WHERE document_id = d.id ORDER BY position, id) AS f
    ), '')
FROM documents AS d;

-- +goose Down
SELECT 1;
