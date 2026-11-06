-- +goose Up

-- A document has an abstract of its own (2026-11-05). The EndNote import and a library's
-- exported citation put one in the notes, as "Abstract: …", alone or after the notes the
-- record had, a blank line between; it moves out of them.
ALTER TABLE documents ADD COLUMN abstract TEXT NOT NULL DEFAULT '';

UPDATE documents SET abstract = trim(substr(notes, 11)), notes = ''
WHERE notes LIKE 'Abstract: %';

UPDATE documents
SET abstract = trim(substr(notes, instr(notes, char(10) || char(10) || 'Abstract: ') + 12)),
    notes = trim(substr(notes, 1, instr(notes, char(10) || char(10) || 'Abstract: ') - 1))
WHERE abstract = '' AND instr(notes, char(10) || char(10) || 'Abstract: ') > 0;

-- The search takes the abstract as a column of its own. An FTS5 table takes no new column,
-- so the index is made again and filled, as the catalog indexes a document.
DROP TABLE document_search;

CREATE VIRTUAL TABLE document_search USING fts5 (
    document_id UNINDEXED, title, authors, container, subjects, abstract, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

INSERT INTO document_search (document_id, title, authors, container, subjects, abstract, notes, body)
SELECT
    CAST(d.id AS TEXT), d.title,
    COALESCE((SELECT group_concat(a.value, '; ') FROM json_each(d.authors) AS a), ''),
    d.container,
    COALESCE((SELECT group_concat(s.value, '; ') FROM json_each(d.subjects) AS s), ''),
    d.abstract, d.notes,
    COALESCE((
        SELECT group_concat(f.text, char(10))
        FROM (SELECT text FROM document_files WHERE document_id = d.id ORDER BY position, id) AS f
    ), '')
FROM documents AS d;

-- +goose Down
SELECT 1;
