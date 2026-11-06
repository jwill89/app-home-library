-- +goose Up

-- An EndNote export brings articles and chapters as documents (2026-11-05). Each keeps the record
-- it came from, as an imported book's copy does, so the same file imported twice adds nothing.
CREATE TABLE document_import_refs (
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    PRIMARY KEY (source, external_id)
);

-- +goose Down
DROP TABLE document_import_refs;
