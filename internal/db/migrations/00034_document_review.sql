-- +goose NO TRANSACTION
-- +goose Up

-- A book review is a kind of paper. SQLite cannot change a CHECK in place, so the
-- table is made again with the kind allowed, by SQLite's own steps: foreign keys off for the
-- move, since dropping the old table would otherwise delete every paper's files with it (they
-- cascade), and the move in a transaction of its own. A transaction cannot turn foreign keys
-- off, so this migration runs outside goose's.
PRAGMA foreign_keys = OFF;

BEGIN IMMEDIATE;

CREATE TABLE documents_new (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'other' CHECK (kind IN ('article', 'chapter', 'review', 'handout', 'syllabus', 'notes', 'exam', 'other')),
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
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    subjects TEXT NOT NULL DEFAULT '[]',
    abstract TEXT NOT NULL DEFAULT '',
    editors TEXT NOT NULL DEFAULT '[]'
);

INSERT INTO documents_new (id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url,
    course_id, notes, shared, created_at, updated_at, subjects, abstract, editors)
SELECT id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url,
    course_id, notes, shared, created_at, updated_at, subjects, abstract, editors
FROM documents;

DROP TABLE documents;
ALTER TABLE documents_new RENAME TO documents;

CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';
CREATE INDEX documents_course_idx ON documents (course_id);

-- +goose StatementBegin
CREATE TRIGGER documents_changed_insert AFTER INSERT ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_update AFTER UPDATE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;
-- +goose StatementEnd

COMMIT;

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

BEGIN IMMEDIATE;

CREATE TABLE documents_old (
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
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    subjects TEXT NOT NULL DEFAULT '[]',
    abstract TEXT NOT NULL DEFAULT '',
    editors TEXT NOT NULL DEFAULT '[]'
);

INSERT INTO documents_old (id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url,
    course_id, notes, shared, created_at, updated_at, subjects, abstract, editors)
SELECT id, CASE kind WHEN 'review' THEN 'article' ELSE kind END, title, authors, container, volume, issue, pages,
    published, publisher, doi, url, course_id, notes, shared, created_at, updated_at, subjects, abstract, editors
FROM documents;

DROP TABLE documents;
ALTER TABLE documents_old RENAME TO documents;

CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';
CREATE INDEX documents_course_idx ON documents (course_id);

-- +goose StatementBegin
CREATE TRIGGER documents_changed_insert AFTER INSERT ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_update AFTER UPDATE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;
-- +goose StatementEnd

COMMIT;

PRAGMA foreign_keys = ON;
