-- +goose Up

-- What the owner kept from school and reading: articles, chapters, handouts, syllabi, notes,
-- exams. The fields are what a citation needs. Authors are a list of names as written, since
-- a handout's author is rarely anyone the books share.
CREATE TABLE documents (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'other' CHECK (kind IN ('article', 'chapter', 'handout', 'syllabus', 'notes', 'exam', 'other')),
    title TEXT NOT NULL,
    authors TEXT NOT NULL DEFAULT '[]',
    container TEXT NOT NULL DEFAULT '',
    volume TEXT NOT NULL DEFAULT '',
    issue TEXT NOT NULL DEFAULT '',
    pages TEXT NOT NULL DEFAULT '',
    published TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    doi TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    course_id INTEGER REFERENCES courses (id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    shared INTEGER NOT NULL DEFAULT 0 CHECK (shared IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';
CREATE INDEX documents_course_idx ON documents (course_id);

-- A document's files, in order: a PDF, or the photographs of its pages. stored is the file's
-- random name in the documents directory. The text is what the text service read from it,
-- or what the owner typed, which the service never replaces.
CREATE TABLE document_files (
    id INTEGER PRIMARY KEY,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    stored TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png', 'image/webp')),
    size INTEGER NOT NULL,
    position INTEGER NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    text_status TEXT NOT NULL DEFAULT 'pending' CHECK (text_status IN ('pending', 'done', 'empty', 'failed', 'owner')),
    text_attempts INTEGER NOT NULL DEFAULT 0,
    text_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX document_files_document_idx ON document_files (document_id, position);
CREATE INDEX document_files_text_idx ON document_files (text_status, id);

-- The full-text index: one row a document, rewritten whenever the document or a file's text
-- changes. The document's id is a column of its own, not the rowid, which sqlc cannot see.
CREATE VIRTUAL TABLE document_search USING fts5 (
    document_id UNINDEXED, title, authors, container, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

-- +goose Down
DROP TABLE document_search;
DROP TABLE document_files;
DROP TABLE documents;
