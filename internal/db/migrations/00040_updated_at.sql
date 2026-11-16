-- +goose NO TRANSACTION
-- +goose Up

-- Every record a form saves whole carries when it last changed, so a save made from a copy
-- older than the one kept is refused rather than undoing the change: borrowers, loans, share
-- links, areas, bookcases and shelves take the updated_at the books, copies, courses and
-- papers already have, starting from when each was made (now, for an area, which kept no
-- such time). A column defaulting to now can't be added in place, so each table is rebuilt
-- as 00038's were, and the loans' view is made again to carry the new column.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);

DROP VIEW loan_rows;

CREATE TABLE borrowers_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO borrowers_new (id, name, contact, notes, created_at, updated_at)
SELECT id, name, contact, notes, created_at, created_at
FROM borrowers;
INSERT INTO rebuild_checks SELECT 'borrowers rows', (SELECT count(*) FROM borrowers) = (SELECT count(*) FROM borrowers_new);
DROP TABLE borrowers;
ALTER TABLE borrowers_new RENAME TO borrowers;

CREATE TABLE loans_new (
    id INTEGER PRIMARY KEY,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    borrower_id INTEGER NOT NULL REFERENCES borrowers (id) ON DELETE CASCADE,
    lent_on DATE NOT NULL CHECK (lent_on IS date(lent_on)),
    due_on DATE CHECK (due_on IS date(due_on)),
    returned_on DATE CHECK (returned_on IS date(returned_on)),
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO loans_new (id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at, updated_at)
SELECT id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at, created_at
FROM loans;
INSERT INTO rebuild_checks SELECT 'loans rows', (SELECT count(*) FROM loans) = (SELECT count(*) FROM loans_new);
DROP TABLE loans;
ALTER TABLE loans_new RENAME TO loans;
CREATE INDEX loans_borrower_idx ON loans (borrower_id);
CREATE INDEX loans_copy_idx ON loans (copy_id);
CREATE UNIQUE INDEX loans_out_idx ON loans (copy_id) WHERE returned_on IS NULL;
-- +goose StatementBegin
CREATE TRIGGER loans_changed_delete AFTER DELETE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', OLD.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = OLD.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_insert AFTER INSERT ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_update AFTER UPDATE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd

CREATE TABLE shares_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at)),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at)),
    books TEXT NOT NULL DEFAULT 'all' CHECK (books IN ('all', 'some', 'none')),
    wishlist INTEGER NOT NULL DEFAULT 1 CHECK (wishlist IN (0, 1)),
    papers TEXT NOT NULL DEFAULT 'none' CHECK (papers IN ('all', 'some', 'none')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO shares_new (id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers, updated_at)
SELECT id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers, created_at
FROM shares;
INSERT INTO rebuild_checks SELECT 'shares rows', (SELECT count(*) FROM shares) = (SELECT count(*) FROM shares_new);
DROP TABLE shares;
ALTER TABLE shares_new RENAME TO shares;

CREATE TABLE areas_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    position INTEGER NOT NULL,
    dewey_from TEXT NOT NULL DEFAULT '',
    dewey_to TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    min_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (min_height_mm BETWEEN 0 AND 1000),
    max_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (max_height_mm BETWEEN 0 AND 1000),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO areas_new (id, name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm, updated_at)
SELECT id, name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm, strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM areas;
INSERT INTO rebuild_checks SELECT 'areas rows', (SELECT count(*) FROM areas) = (SELECT count(*) FROM areas_new);
DROP TABLE areas;
ALTER TABLE areas_new RENAME TO areas;

CREATE TABLE bookcases_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO bookcases_new (id, name, position, created_at, updated_at)
SELECT id, name, position, created_at, created_at
FROM bookcases;
INSERT INTO rebuild_checks SELECT 'bookcases rows', (SELECT count(*) FROM bookcases) = (SELECT count(*) FROM bookcases_new);
DROP TABLE bookcases;
ALTER TABLE bookcases_new RENAME TO bookcases;

CREATE TABLE shelves_new (
    id INTEGER PRIMARY KEY,
    bookcase_id INTEGER NOT NULL REFERENCES bookcases (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 100 AND 5000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 50 AND 1000),
    depth_mm INTEGER NOT NULL CHECK (depth_mm BETWEEN 50 AND 1000),
    starts_at TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200),
    brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200),
    area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL,
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO shelves_new (id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id, updated_at)
SELECT id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id, created_at
FROM shelves;
INSERT INTO rebuild_checks SELECT 'shelves rows', (SELECT count(*) FROM shelves) = (SELECT count(*) FROM shelves_new);
DROP TABLE shelves;
ALTER TABLE shelves_new RENAME TO shelves;
CREATE INDEX shelves_area_idx ON shelves (area_id);
CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

CREATE VIEW loan_rows AS
SELECT
    l.id, l.copy_id, l.borrower_id, b.name AS borrower, l.lent_on, l.due_on, l.returned_on,
    l.notes, l.updated_at, c.edition_id, e.title, e.cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM loans l
JOIN borrowers b ON b.id = l.borrower_id
JOIN copies c ON c.id = l.copy_id
JOIN editions e ON e.id = c.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = c.edition_id;

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
PRAGMA user_version = 40;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;

-- +goose Down
-- Each table as it was, without when it last changed.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);

DROP VIEW loan_rows;

CREATE TABLE borrowers_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO borrowers_old (id, name, contact, notes, created_at)
SELECT id, name, contact, notes, created_at
FROM borrowers;
INSERT INTO rebuild_checks SELECT 'borrowers rows', (SELECT count(*) FROM borrowers) = (SELECT count(*) FROM borrowers_old);
DROP TABLE borrowers;
ALTER TABLE borrowers_old RENAME TO borrowers;

CREATE TABLE loans_old (
    id INTEGER PRIMARY KEY,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    borrower_id INTEGER NOT NULL REFERENCES borrowers (id) ON DELETE CASCADE,
    lent_on DATE NOT NULL
        CHECK (lent_on IS date(lent_on)),
    due_on DATE
        CHECK (due_on IS date(due_on)),
    returned_on DATE
        CHECK (returned_on IS date(returned_on)),
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO loans_old (id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at)
SELECT id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at
FROM loans;
INSERT INTO rebuild_checks SELECT 'loans rows', (SELECT count(*) FROM loans) = (SELECT count(*) FROM loans_old);
DROP TABLE loans;
ALTER TABLE loans_old RENAME TO loans;
CREATE INDEX loans_borrower_idx ON loans (borrower_id);
CREATE INDEX loans_copy_idx ON loans (copy_id);
CREATE UNIQUE INDEX loans_out_idx ON loans (copy_id) WHERE returned_on IS NULL;
-- +goose StatementBegin
CREATE TRIGGER loans_changed_delete AFTER DELETE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', OLD.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = OLD.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_insert AFTER INSERT ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_update AFTER UPDATE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd

CREATE TABLE shares_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME
        CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at)),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME
        CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at)),
    books TEXT NOT NULL DEFAULT 'all' CHECK (books IN ('all', 'some', 'none')),
    wishlist INTEGER NOT NULL DEFAULT 1 CHECK (wishlist IN (0, 1)),
    papers TEXT NOT NULL DEFAULT 'none' CHECK (papers IN ('all', 'some', 'none'))
);
INSERT INTO shares_old (id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers)
SELECT id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers
FROM shares;
INSERT INTO rebuild_checks SELECT 'shares rows', (SELECT count(*) FROM shares) = (SELECT count(*) FROM shares_old);
DROP TABLE shares;
ALTER TABLE shares_old RENAME TO shares;

CREATE TABLE areas_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    position INTEGER NOT NULL,
    dewey_from TEXT NOT NULL DEFAULT '',
    dewey_to TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    min_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (min_height_mm BETWEEN 0 AND 1000),
    max_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (max_height_mm BETWEEN 0 AND 1000)
);
INSERT INTO areas_old (id, name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm)
SELECT id, name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm
FROM areas;
INSERT INTO rebuild_checks SELECT 'areas rows', (SELECT count(*) FROM areas) = (SELECT count(*) FROM areas_old);
DROP TABLE areas;
ALTER TABLE areas_old RENAME TO areas;

CREATE TABLE bookcases_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO bookcases_old (id, name, position, created_at)
SELECT id, name, position, created_at
FROM bookcases;
INSERT INTO rebuild_checks SELECT 'bookcases rows', (SELECT count(*) FROM bookcases) = (SELECT count(*) FROM bookcases_old);
DROP TABLE bookcases;
ALTER TABLE bookcases_old RENAME TO bookcases;

CREATE TABLE shelves_old (
    id INTEGER PRIMARY KEY,
    bookcase_id INTEGER NOT NULL REFERENCES bookcases (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 100 AND 5000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 50 AND 1000),
    depth_mm INTEGER NOT NULL CHECK (depth_mm BETWEEN 50 AND 1000),
    starts_at TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200),
    brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200),
    area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL
);
INSERT INTO shelves_old (id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id)
SELECT id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id
FROM shelves;
INSERT INTO rebuild_checks SELECT 'shelves rows', (SELECT count(*) FROM shelves) = (SELECT count(*) FROM shelves_old);
DROP TABLE shelves;
ALTER TABLE shelves_old RENAME TO shelves;
CREATE INDEX shelves_area_idx ON shelves (area_id);
CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

CREATE VIEW loan_rows AS
SELECT
    l.id, l.copy_id, l.borrower_id, b.name AS borrower, l.lent_on, l.due_on, l.returned_on,
    l.notes, c.edition_id, e.title, e.cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM loans l
JOIN borrowers b ON b.id = l.borrower_id
JOIN copies c ON c.id = l.copy_id
JOIN editions e ON e.id = c.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = c.edition_id;

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
PRAGMA user_version = 39;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;
