-- +goose Up

-- The search index's row is the document's own id, so rewriting one document's entry finds
-- it by rowid. Keyed by a column of its own, which FTS5 cannot index, every save scanned the
-- text of every document to find the row to replace.
DROP TABLE document_search;

CREATE VIRTUAL TABLE document_search USING fts5 (
    title, authors, container, subjects, abstract, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

INSERT INTO document_search (rowid, title, authors, container, subjects, abstract, notes, body)
SELECT
    d.id, d.title,
    COALESCE((
        SELECT group_concat(n.value, '; ')
        FROM (SELECT value FROM json_each(d.authors) UNION ALL SELECT value FROM json_each(d.editors)) AS n
    ), ''),
    d.container,
    COALESCE((SELECT group_concat(s.value, '; ') FROM json_each(d.subjects) AS s), ''),
    d.abstract, d.notes,
    COALESCE((
        SELECT group_concat(f.text, char(10))
        FROM (SELECT text FROM document_files WHERE document_id = d.id ORDER BY position, id) AS f
    ), '')
FROM documents AS d;

-- +goose Down
DROP TABLE document_search;

CREATE VIRTUAL TABLE document_search USING fts5 (
    document_id UNINDEXED, title, authors, container, subjects, abstract, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

INSERT INTO document_search (document_id, title, authors, container, subjects, abstract, notes, body)
SELECT
    CAST(d.id AS TEXT), d.title,
    COALESCE((
        SELECT group_concat(n.value, '; ')
        FROM (SELECT value FROM json_each(d.authors) UNION ALL SELECT value FROM json_each(d.editors)) AS n
    ), ''),
    d.container,
    COALESCE((SELECT group_concat(s.value, '; ') FROM json_each(d.subjects) AS s), ''),
    d.abstract, d.notes,
    COALESCE((
        SELECT group_concat(f.text, char(10))
        FROM (SELECT text FROM document_files WHERE document_id = d.id ORDER BY position, id) AS f
    ), '')
FROM documents AS d;
