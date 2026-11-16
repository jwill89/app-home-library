-- +goose NO TRANSACTION
-- +goose Up

-- Every date and time takes its declared type and a CHECK on its one form: a timestamp is
-- DATETIME, RFC 3339 in UTC to the second (2026-11-07T09:18:09Z), and a day is DATE
-- (2026-11-07). A date or time not set is NULL, where it was ''. The change feed's times
-- drop their milliseconds: its ids keep the order.
--
-- SQLite can't change a column's type or add a CHECK, so each table is rebuilt: made anew,
-- its rows copied, the old one dropped and the new one renamed, then its indexes and
-- triggers made again. Foreign keys are off for it, since dropping a parent table with them
-- on deletes its children's rows, and legacy_alter_table is on, so a rename leaves the other
-- tables' triggers and the views as they are: every name ends where it began. The whole
-- rebuild is one transaction; a failed check stops it before it commits.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

-- Each row says whether one thing held: every row copied, every date and time kept, no
-- reference left without its row.
CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_dates_and_references_all_kept CHECK (ok)
);

-- The AUTOINCREMENT tables go on from the highest id they ever gave, not the highest kept.
CREATE TEMP TABLE kept_sequence AS
SELECT name, seq FROM sqlite_sequence WHERE name IN ('copies', 'changes');

CREATE TABLE users_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO users_new (id, name, created_at)
SELECT id, name, strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
FROM users;
INSERT INTO rebuild_checks SELECT 'users rows', (SELECT count(*) FROM users) = (SELECT count(*) FROM users_new);
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

CREATE TABLE passkeys_new (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    credential_id TEXT NOT NULL UNIQUE,
    credential TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME
        CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at))
);
INSERT INTO passkeys_new (id, user_id, rp_id, credential_id, credential, name, created_at, last_used_at)
SELECT id, user_id, rp_id, credential_id, credential, name, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(last_used_at, ''))
FROM passkeys;
INSERT INTO rebuild_checks SELECT 'passkeys rows', (SELECT count(*) FROM passkeys) = (SELECT count(*) FROM passkeys_new);
INSERT INTO rebuild_checks SELECT 'passkeys.last_used_at', (SELECT count(*) FROM passkeys WHERE last_used_at <> '') = (SELECT count(*) FROM passkeys_new WHERE last_used_at IS NOT NULL);
DROP TABLE passkeys;
ALTER TABLE passkeys_new RENAME TO passkeys;
CREATE INDEX passkeys_user_idx ON passkeys (user_id);

CREATE TABLE setup_codes_new (
    code_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
        CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at))
);
INSERT INTO setup_codes_new (code_hash, user_id, expires_at)
SELECT code_hash, user_id, strftime('%Y-%m-%dT%H:%M:%SZ', expires_at)
FROM setup_codes;
INSERT INTO rebuild_checks SELECT 'setup_codes rows', (SELECT count(*) FROM setup_codes) = (SELECT count(*) FROM setup_codes_new);
DROP TABLE setup_codes;
ALTER TABLE setup_codes_new RENAME TO setup_codes;

CREATE TABLE series_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    dismissed_on DATE
        CHECK (dismissed_on IS date(dismissed_on))
);
INSERT INTO series_new (id, name, dismissed_on)
SELECT id, name, date(NULLIF(dismissed_on, ''))
FROM series;
INSERT INTO rebuild_checks SELECT 'series rows', (SELECT count(*) FROM series) = (SELECT count(*) FROM series_new);
INSERT INTO rebuild_checks SELECT 'series.dismissed_on', (SELECT count(*) FROM series WHERE dismissed_on <> '') = (SELECT count(*) FROM series_new WHERE dismissed_on IS NOT NULL);
DROP TABLE series;
ALTER TABLE series_new RENAME TO series;
-- +goose StatementBegin
CREATE TRIGGER series_changed AFTER UPDATE ON series BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e JOIN works w ON w.id = e.work_id WHERE w.series_id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE works_new (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    series_id INTEGER REFERENCES series (id) ON DELETE SET NULL,
    series_number TEXT NOT NULL DEFAULT '',
    read_status TEXT NOT NULL DEFAULT 'unread' CHECK (read_status IN ('unread', 'reading', 'read')),
    current_page INTEGER,
    started_on DATE
        CHECK (started_on IS date(started_on)),
    finished_on DATE
        CHECK (finished_on IS date(finished_on)),
    rating INTEGER CHECK (rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    category TEXT NOT NULL DEFAULT '' CHECK (category IN ('', 'fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized'))
);
INSERT INTO works_new (id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at, category)
SELECT id, title, series_id, series_number, read_status, current_page, date(NULLIF(started_on, '')), date(NULLIF(finished_on, '')), rating, notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), category
FROM works;
INSERT INTO rebuild_checks SELECT 'works rows', (SELECT count(*) FROM works) = (SELECT count(*) FROM works_new);
INSERT INTO rebuild_checks SELECT 'works.started_on', (SELECT count(*) FROM works WHERE started_on <> '') = (SELECT count(*) FROM works_new WHERE started_on IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'works.finished_on', (SELECT count(*) FROM works WHERE finished_on <> '') = (SELECT count(*) FROM works_new WHERE finished_on IS NOT NULL);
DROP TABLE works;
ALTER TABLE works_new RENAME TO works;
CREATE INDEX works_series_idx ON works (series_id);
-- +goose StatementBegin
CREATE TRIGGER works_changed AFTER UPDATE ON works BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE editions_new (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    isbn13 TEXT UNIQUE,
    isbn10 TEXT NOT NULL DEFAULT '',
    lccn TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    publish_place TEXT NOT NULL DEFAULT '',
    published TEXT NOT NULL DEFAULT '',
    edition_statement TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '' CHECK (format IN ('', 'hardcover', 'paperback', 'trade_paperback', 'mass_market', 'library_binding', 'spiral', 'board_book', 'leather', 'other')),
    pages INTEGER,
    height_mm INTEGER,
    width_mm INTEGER,
    thickness_mm INTEGER,
    language TEXT NOT NULL DEFAULT '',
    ddc TEXT NOT NULL DEFAULT '',
    lcc TEXT NOT NULL DEFAULT '',
    subjects TEXT NOT NULL DEFAULT '[]',
    description TEXT NOT NULL DEFAULT '',
    cover TEXT NOT NULL DEFAULT '',
    sources TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    wanted_on DATE
        CHECK (wanted_on IS date(wanted_on))
);
INSERT INTO editions_new (id, work_id, isbn13, isbn10, lccn, title, subtitle, publisher, publish_place, published, edition_statement, format, pages, height_mm, width_mm, thickness_mm, language, ddc, lcc, subjects, description, cover, sources, created_at, updated_at, wanted_on)
SELECT id, work_id, isbn13, isbn10, lccn, title, subtitle, publisher, publish_place, published, edition_statement, format, pages, height_mm, width_mm, thickness_mm, language, ddc, lcc, subjects, description, cover, sources, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), date(NULLIF(wanted_on, ''))
FROM editions;
INSERT INTO rebuild_checks SELECT 'editions rows', (SELECT count(*) FROM editions) = (SELECT count(*) FROM editions_new);
INSERT INTO rebuild_checks SELECT 'editions.wanted_on', (SELECT count(*) FROM editions WHERE wanted_on <> '') = (SELECT count(*) FROM editions_new WHERE wanted_on IS NOT NULL);
DROP TABLE editions;
ALTER TABLE editions_new RENAME TO editions;
CREATE INDEX editions_work_idx ON editions (work_id);
-- +goose StatementBegin
CREATE TRIGGER editions_changed_delete AFTER DELETE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_insert AFTER INSERT ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_update AFTER UPDATE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd

CREATE TABLE copies_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    section_id INTEGER REFERENCES sections (id) ON DELETE SET NULL,
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    acquired_on DATE
        CHECK (acquired_on IS date(acquired_on)),
    acquired_from TEXT NOT NULL DEFAULT '',
    price_cents INTEGER,
    condition TEXT NOT NULL DEFAULT '' CHECK (condition IN ('', 'fine', 'very_good', 'good', 'fair', 'poor')),
    dust_jacket INTEGER CHECK (dust_jacket IN (0, 1)),
    signed INTEGER NOT NULL DEFAULT 0 CHECK (signed IN (0, 1)),
    location TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    gone_as TEXT NOT NULL DEFAULT '' CHECK (gone_as IN ('', 'lost', 'sold', 'donated', 'given', 'discarded')),
    gone_on DATE
        CHECK (gone_on IS date(gone_on)),
    gone_note TEXT NOT NULL DEFAULT '',
    seen_on DATE
        CHECK (seen_on IS date(seen_on))
);
INSERT INTO copies_new (id, edition_id, section_id, acquired_as, acquired_on, acquired_from, price_cents, condition, dust_jacket, signed, location, notes, created_at, updated_at, gone_as, gone_on, gone_note, seen_on)
SELECT id, edition_id, section_id, acquired_as, date(NULLIF(acquired_on, '')), acquired_from, price_cents, condition, dust_jacket, signed, location, notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), gone_as, date(NULLIF(gone_on, '')), gone_note, date(NULLIF(seen_on, ''))
FROM copies;
INSERT INTO rebuild_checks SELECT 'copies rows', (SELECT count(*) FROM copies) = (SELECT count(*) FROM copies_new);
INSERT INTO rebuild_checks SELECT 'copies.acquired_on', (SELECT count(*) FROM copies WHERE acquired_on <> '') = (SELECT count(*) FROM copies_new WHERE acquired_on IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'copies.gone_on', (SELECT count(*) FROM copies WHERE gone_on <> '') = (SELECT count(*) FROM copies_new WHERE gone_on IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'copies.seen_on', (SELECT count(*) FROM copies WHERE seen_on <> '') = (SELECT count(*) FROM copies_new WHERE seen_on IS NOT NULL);
DROP TABLE copies;
ALTER TABLE copies_new RENAME TO copies;
CREATE INDEX copies_edition_idx ON copies (edition_id);
-- +goose StatementBegin
CREATE TRIGGER copies_changed_delete AFTER DELETE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_insert AFTER INSERT ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_update AFTER UPDATE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd

CREATE TABLE enrichment_new (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (next_at IS strftime('%Y-%m-%dT%H:%M:%SZ', next_at)),
    last_error TEXT NOT NULL DEFAULT '',
    queued_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (queued_at IS strftime('%Y-%m-%dT%H:%M:%SZ', queued_at)),
    fresh INTEGER NOT NULL DEFAULT 0 CHECK (fresh IN (0, 1))
);
INSERT INTO enrichment_new (edition_id, status, attempts, next_at, last_error, queued_at, fresh)
SELECT edition_id, status, attempts, strftime('%Y-%m-%dT%H:%M:%SZ', next_at), last_error, strftime('%Y-%m-%dT%H:%M:%SZ', queued_at), fresh
FROM enrichment;
INSERT INTO rebuild_checks SELECT 'enrichment rows', (SELECT count(*) FROM enrichment) = (SELECT count(*) FROM enrichment_new);
DROP TABLE enrichment;
ALTER TABLE enrichment_new RENAME TO enrichment;
CREATE INDEX enrichment_due_idx ON enrichment (status, next_at);
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_delete AFTER DELETE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_insert AFTER INSERT ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_update AFTER UPDATE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd

CREATE TABLE lookups_new (
    lookup_key TEXT NOT NULL,
    source TEXT NOT NULL,
    fetched_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (fetched_at IS strftime('%Y-%m-%dT%H:%M:%SZ', fetched_at)),
    body BLOB NOT NULL,
    PRIMARY KEY (lookup_key, source)
);
INSERT INTO lookups_new (lookup_key, source, fetched_at, body)
SELECT lookup_key, source, strftime('%Y-%m-%dT%H:%M:%SZ', fetched_at), body
FROM lookups;
INSERT INTO rebuild_checks SELECT 'lookups rows', (SELECT count(*) FROM lookups) = (SELECT count(*) FROM lookups_new);
DROP TABLE lookups;
ALTER TABLE lookups_new RENAME TO lookups;

CREATE TABLE shares_new (
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
INSERT INTO shares_new (id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers)
SELECT id, name, token_hash, strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(expires_at, '')), strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(last_used_at, '')), books, wishlist, papers
FROM shares;
INSERT INTO rebuild_checks SELECT 'shares rows', (SELECT count(*) FROM shares) = (SELECT count(*) FROM shares_new);
INSERT INTO rebuild_checks SELECT 'shares.expires_at', (SELECT count(*) FROM shares WHERE expires_at <> '') = (SELECT count(*) FROM shares_new WHERE expires_at IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'shares.last_used_at', (SELECT count(*) FROM shares WHERE last_used_at <> '') = (SELECT count(*) FROM shares_new WHERE last_used_at IS NOT NULL);
DROP TABLE shares;
ALTER TABLE shares_new RENAME TO shares;

CREATE TABLE borrowers_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO borrowers_new (id, name, contact, notes, created_at)
SELECT id, name, contact, notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
FROM borrowers;
INSERT INTO rebuild_checks SELECT 'borrowers rows', (SELECT count(*) FROM borrowers) = (SELECT count(*) FROM borrowers_new);
DROP TABLE borrowers;
ALTER TABLE borrowers_new RENAME TO borrowers;

CREATE TABLE loans_new (
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
INSERT INTO loans_new (id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at)
SELECT id, copy_id, borrower_id, date(lent_on), date(NULLIF(due_on, '')), date(NULLIF(returned_on, '')), notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
FROM loans;
INSERT INTO rebuild_checks SELECT 'loans rows', (SELECT count(*) FROM loans) = (SELECT count(*) FROM loans_new);
INSERT INTO rebuild_checks SELECT 'loans.due_on', (SELECT count(*) FROM loans WHERE due_on <> '') = (SELECT count(*) FROM loans_new WHERE due_on IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'loans.returned_on', (SELECT count(*) FROM loans WHERE returned_on <> '') = (SELECT count(*) FROM loans_new WHERE returned_on IS NOT NULL);
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

CREATE TABLE courses_new (
    id INTEGER PRIMARY KEY,
    school TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT '' CHECK (level IN ('', 'high_school', 'undergraduate', 'graduate', 'professional', 'other')),
    department TEXT NOT NULL DEFAULT '',
    number TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    term TEXT NOT NULL DEFAULT '' CHECK (term IN ('', 'winter', 'spring', 'summer', 'fall')),
    year INTEGER CHECK (year BETWEEN 1900 AND 2100),
    instructor TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);
INSERT INTO courses_new (id, school, level, department, number, title, term, year, instructor, notes, created_at, updated_at)
SELECT id, school, level, department, number, title, term, year, instructor, notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)
FROM courses;
INSERT INTO rebuild_checks SELECT 'courses rows', (SELECT count(*) FROM courses) = (SELECT count(*) FROM courses_new);
DROP TABLE courses;
ALTER TABLE courses_new RENAME TO courses;

CREATE TABLE label_queue_new (
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('barcode', 'spine')),
    queued_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (queued_at IS strftime('%Y-%m-%dT%H:%M:%SZ', queued_at)),
    PRIMARY KEY (copy_id, kind)
);
INSERT INTO label_queue_new (copy_id, kind, queued_at)
SELECT copy_id, kind, strftime('%Y-%m-%dT%H:%M:%SZ', queued_at)
FROM label_queue;
INSERT INTO rebuild_checks SELECT 'label_queue rows', (SELECT count(*) FROM label_queue) = (SELECT count(*) FROM label_queue_new);
DROP TABLE label_queue;
ALTER TABLE label_queue_new RENAME TO label_queue;
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_delete AFTER DELETE ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', OLD.copy_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_insert AFTER INSERT ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', NEW.copy_id);
END;
-- +goose StatementEnd

CREATE TABLE document_files_new (
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
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO document_files_new (id, document_id, name, stored, content_type, size, position, text, text_status, text_attempts, text_error, created_at)
SELECT id, document_id, name, stored, content_type, size, position, text, text_status, text_attempts, text_error, strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
FROM document_files;
INSERT INTO rebuild_checks SELECT 'document_files rows', (SELECT count(*) FROM document_files) = (SELECT count(*) FROM document_files_new);
DROP TABLE document_files;
ALTER TABLE document_files_new RENAME TO document_files;
CREATE INDEX document_files_document_idx ON document_files (document_id, position);
CREATE INDEX document_files_text_idx ON document_files (text_status, id);
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_delete AFTER DELETE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_insert AFTER INSERT ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_update AFTER UPDATE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd

CREATE TABLE bookcases_new (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO bookcases_new (id, name, position, created_at)
SELECT id, name, position, strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
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
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200),
    brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200),
    area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL
);
INSERT INTO shelves_new (id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id)
SELECT id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), brace_height_mm, brace_width_mm, area_id
FROM shelves;
INSERT INTO rebuild_checks SELECT 'shelves rows', (SELECT count(*) FROM shelves) = (SELECT count(*) FROM shelves_new);
DROP TABLE shelves;
ALTER TABLE shelves_new RENAME TO shelves;
CREATE INDEX shelves_area_idx ON shelves (area_id);
CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

CREATE TABLE layout_settings_new (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    fill_percent INTEGER NOT NULL DEFAULT 80 CHECK (fill_percent BETWEEN 50 AND 100),
    applied_at DATETIME
        CHECK (applied_at IS strftime('%Y-%m-%dT%H:%M:%SZ', applied_at))
);
INSERT INTO layout_settings_new (id, fill_percent, applied_at)
SELECT id, fill_percent, strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(applied_at, ''))
FROM layout_settings;
INSERT INTO rebuild_checks SELECT 'layout_settings rows', (SELECT count(*) FROM layout_settings) = (SELECT count(*) FROM layout_settings_new);
INSERT INTO rebuild_checks SELECT 'layout_settings.applied_at', (SELECT count(*) FROM layout_settings WHERE applied_at <> '') = (SELECT count(*) FROM layout_settings_new WHERE applied_at IS NOT NULL);
DROP TABLE layout_settings;
ALTER TABLE layout_settings_new RENAME TO layout_settings;

CREATE TABLE scans_new (
    id INTEGER PRIMARY KEY,
    isbn13 TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'found', 'adding', 'added', 'owned', 'not_found', 'failed')),
    proposal TEXT NOT NULL DEFAULT '',
    edition_id INTEGER REFERENCES editions (id) ON DELETE SET NULL,
    error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    reviewed INTEGER NOT NULL DEFAULT 0 CHECK (reviewed IN (0, 1)),
    scanned_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (scanned_at IS strftime('%Y-%m-%dT%H:%M:%SZ', scanned_at)),
    done_at DATETIME
        CHECK (done_at IS strftime('%Y-%m-%dT%H:%M:%SZ', done_at)),
    want INTEGER NOT NULL DEFAULT 0 CHECK (want IN (0, 1))
);
INSERT INTO scans_new (id, isbn13, section, acquired_as, status, proposal, edition_id, error, attempts, reviewed, scanned_at, done_at, want)
SELECT id, isbn13, section, acquired_as, status, proposal, edition_id, error, attempts, reviewed, strftime('%Y-%m-%dT%H:%M:%SZ', scanned_at), strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(done_at, '')), want
FROM scans;
INSERT INTO rebuild_checks SELECT 'scans rows', (SELECT count(*) FROM scans) = (SELECT count(*) FROM scans_new);
INSERT INTO rebuild_checks SELECT 'scans.done_at', (SELECT count(*) FROM scans WHERE done_at <> '') = (SELECT count(*) FROM scans_new WHERE done_at IS NOT NULL);
DROP TABLE scans;
ALTER TABLE scans_new RENAME TO scans;
CREATE INDEX scans_open_idx ON scans (reviewed, id);
CREATE INDEX scans_status_idx ON scans (status, id);
-- +goose StatementBegin
CREATE TRIGGER scans_changed_delete AFTER DELETE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_insert AFTER INSERT ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_update AFTER UPDATE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd

CREATE TABLE readings_new (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    started_on DATE
        CHECK (started_on IS date(started_on)),
    finished_on DATE
        CHECK (finished_on IS date(finished_on)),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);
INSERT INTO readings_new (id, work_id, started_on, finished_on, created_at)
SELECT id, work_id, date(NULLIF(started_on, '')), date(NULLIF(finished_on, '')), strftime('%Y-%m-%dT%H:%M:%SZ', created_at)
FROM readings;
INSERT INTO rebuild_checks SELECT 'readings rows', (SELECT count(*) FROM readings) = (SELECT count(*) FROM readings_new);
INSERT INTO rebuild_checks SELECT 'readings.started_on', (SELECT count(*) FROM readings WHERE started_on <> '') = (SELECT count(*) FROM readings_new WHERE started_on IS NOT NULL);
INSERT INTO rebuild_checks SELECT 'readings.finished_on', (SELECT count(*) FROM readings WHERE finished_on <> '') = (SELECT count(*) FROM readings_new WHERE finished_on IS NOT NULL);
DROP TABLE readings;
ALTER TABLE readings_new RENAME TO readings;
CREATE INDEX readings_work_idx ON readings (work_id, finished_on);
-- +goose StatementBegin
CREATE TRIGGER readings_changed_delete AFTER DELETE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = OLD.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_insert AFTER INSERT ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_update AFTER UPDATE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd

CREATE TABLE calendar_links_new (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME
        CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at))
);
INSERT INTO calendar_links_new (id, token_hash, created_at, last_used_at)
SELECT id, token_hash, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', NULLIF(last_used_at, ''))
FROM calendar_links;
INSERT INTO rebuild_checks SELECT 'calendar_links rows', (SELECT count(*) FROM calendar_links) = (SELECT count(*) FROM calendar_links_new);
INSERT INTO rebuild_checks SELECT 'calendar_links.last_used_at', (SELECT count(*) FROM calendar_links WHERE last_used_at <> '') = (SELECT count(*) FROM calendar_links_new WHERE last_used_at IS NOT NULL);
DROP TABLE calendar_links;
ALTER TABLE calendar_links_new RENAME TO calendar_links;

CREATE TABLE changes_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('book', 'scan', 'document', 'lookup', 'loan', 'label')),
    ref INTEGER NOT NULL,
    at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (at IS strftime('%Y-%m-%dT%H:%M:%SZ', at))
);
INSERT INTO changes_new (id, type, ref, at)
SELECT id, type, ref, strftime('%Y-%m-%dT%H:%M:%SZ', at)
FROM changes;
INSERT INTO rebuild_checks SELECT 'changes rows', (SELECT count(*) FROM changes) = (SELECT count(*) FROM changes_new);
DROP TABLE changes;
ALTER TABLE changes_new RENAME TO changes;

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
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    subjects TEXT NOT NULL DEFAULT '[]',
    abstract TEXT NOT NULL DEFAULT '',
    editors TEXT NOT NULL DEFAULT '[]'
);
INSERT INTO documents_new (id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url, course_id, notes, created_at, updated_at, subjects, abstract, editors)
SELECT id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url, course_id, notes, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), strftime('%Y-%m-%dT%H:%M:%SZ', updated_at), subjects, abstract, editors
FROM documents;
INSERT INTO rebuild_checks SELECT 'documents rows', (SELECT count(*) FROM documents) = (SELECT count(*) FROM documents_new);
DROP TABLE documents;
ALTER TABLE documents_new RENAME TO documents;
CREATE INDEX documents_course_idx ON documents (course_id);
CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';
-- +goose StatementBegin
CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;
-- +goose StatementEnd
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

INSERT INTO sqlite_sequence (name, seq)
SELECT k.name, k.seq FROM temp.kept_sequence k
WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence s WHERE s.name = k.name);
UPDATE sqlite_sequence
SET seq = max(seq, (SELECT k.seq FROM temp.kept_sequence k WHERE k.name = sqlite_sequence.name))
WHERE name IN (SELECT name FROM temp.kept_sequence);

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
DROP TABLE temp.kept_sequence;
PRAGMA user_version = 38;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;

-- +goose Down
-- Each table as it was, its days and times text again, with an empty string for one not set.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);
CREATE TEMP TABLE kept_sequence AS
SELECT name, seq FROM sqlite_sequence WHERE name IN ('copies', 'changes');

CREATE TABLE users_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
INSERT INTO users_old (id, name, created_at)
SELECT id, name, created_at
FROM users;
INSERT INTO rebuild_checks SELECT 'users rows', (SELECT count(*) FROM users) = (SELECT count(*) FROM users_old);
DROP TABLE users;
ALTER TABLE users_old RENAME TO users;

CREATE TABLE passkeys_old (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    credential_id TEXT NOT NULL UNIQUE,
    credential TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT
);
INSERT INTO passkeys_old (id, user_id, rp_id, credential_id, credential, name, created_at, last_used_at)
SELECT id, user_id, rp_id, credential_id, credential, name, created_at, last_used_at
FROM passkeys;
INSERT INTO rebuild_checks SELECT 'passkeys rows', (SELECT count(*) FROM passkeys) = (SELECT count(*) FROM passkeys_old);
DROP TABLE passkeys;
ALTER TABLE passkeys_old RENAME TO passkeys;
CREATE INDEX passkeys_user_idx ON passkeys (user_id);

CREATE TABLE setup_codes_old (
    code_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL
);
INSERT INTO setup_codes_old (code_hash, user_id, expires_at)
SELECT code_hash, user_id, expires_at
FROM setup_codes;
INSERT INTO rebuild_checks SELECT 'setup_codes rows', (SELECT count(*) FROM setup_codes) = (SELECT count(*) FROM setup_codes_old);
DROP TABLE setup_codes;
ALTER TABLE setup_codes_old RENAME TO setup_codes;

CREATE TABLE series_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
, dismissed_on TEXT NOT NULL DEFAULT '');
INSERT INTO series_old (id, name, dismissed_on)
SELECT id, name, COALESCE(dismissed_on, '')
FROM series;
INSERT INTO rebuild_checks SELECT 'series rows', (SELECT count(*) FROM series) = (SELECT count(*) FROM series_old);
DROP TABLE series;
ALTER TABLE series_old RENAME TO series;
-- +goose StatementBegin
CREATE TRIGGER series_changed AFTER UPDATE ON series BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e JOIN works w ON w.id = e.work_id WHERE w.series_id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE works_old (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    series_id INTEGER REFERENCES series (id) ON DELETE SET NULL,
    series_number TEXT NOT NULL DEFAULT '',
    read_status TEXT NOT NULL DEFAULT 'unread' CHECK (read_status IN ('unread', 'reading', 'read')),
    current_page INTEGER,
    started_on TEXT NOT NULL DEFAULT '',
    finished_on TEXT NOT NULL DEFAULT '',
    rating INTEGER CHECK (rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
, category TEXT NOT NULL DEFAULT ''
    CHECK (category IN ('', 'fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')));
INSERT INTO works_old (id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at, category)
SELECT id, title, series_id, COALESCE(series_number, ''), read_status, current_page, COALESCE(started_on, ''), COALESCE(finished_on, ''), rating, COALESCE(notes, ''), created_at, updated_at, COALESCE(category, '')
FROM works;
INSERT INTO rebuild_checks SELECT 'works rows', (SELECT count(*) FROM works) = (SELECT count(*) FROM works_old);
DROP TABLE works;
ALTER TABLE works_old RENAME TO works;
CREATE INDEX works_series_idx ON works (series_id);
-- +goose StatementBegin
CREATE TRIGGER works_changed AFTER UPDATE ON works BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE editions_old (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    isbn13 TEXT UNIQUE,
    isbn10 TEXT NOT NULL DEFAULT '',
    lccn TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    publish_place TEXT NOT NULL DEFAULT '',
    published TEXT NOT NULL DEFAULT '',
    edition_statement TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '' CHECK (format IN ('', 'hardcover', 'paperback', 'trade_paperback', 'mass_market', 'library_binding', 'spiral', 'board_book', 'leather', 'other')),
    pages INTEGER,
    height_mm INTEGER,
    width_mm INTEGER,
    thickness_mm INTEGER,
    language TEXT NOT NULL DEFAULT '',
    ddc TEXT NOT NULL DEFAULT '',
    lcc TEXT NOT NULL DEFAULT '',
    subjects TEXT NOT NULL DEFAULT '[]',
    description TEXT NOT NULL DEFAULT '',
    cover TEXT NOT NULL DEFAULT '',
    sources TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
, wanted_on TEXT NOT NULL DEFAULT '');
INSERT INTO editions_old (id, work_id, isbn13, isbn10, lccn, title, subtitle, publisher, publish_place, published, edition_statement, format, pages, height_mm, width_mm, thickness_mm, language, ddc, lcc, subjects, description, cover, sources, created_at, updated_at, wanted_on)
SELECT id, work_id, isbn13, COALESCE(isbn10, ''), COALESCE(lccn, ''), title, COALESCE(subtitle, ''), COALESCE(publisher, ''), COALESCE(publish_place, ''), COALESCE(published, ''), COALESCE(edition_statement, ''), COALESCE(format, ''), pages, height_mm, width_mm, thickness_mm, COALESCE(language, ''), COALESCE(ddc, ''), COALESCE(lcc, ''), subjects, COALESCE(description, ''), COALESCE(cover, ''), sources, created_at, updated_at, COALESCE(wanted_on, '')
FROM editions;
INSERT INTO rebuild_checks SELECT 'editions rows', (SELECT count(*) FROM editions) = (SELECT count(*) FROM editions_old);
DROP TABLE editions;
ALTER TABLE editions_old RENAME TO editions;
CREATE INDEX editions_work_idx ON editions (work_id);
-- +goose StatementBegin
CREATE TRIGGER editions_changed_delete AFTER DELETE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_insert AFTER INSERT ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_update AFTER UPDATE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd

CREATE TABLE copies_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    section_id INTEGER REFERENCES sections (id) ON DELETE SET NULL,
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    acquired_on TEXT NOT NULL DEFAULT '',
    acquired_from TEXT NOT NULL DEFAULT '',
    price_cents INTEGER,
    condition TEXT NOT NULL DEFAULT '' CHECK (condition IN ('', 'fine', 'very_good', 'good', 'fair', 'poor')),
    dust_jacket INTEGER CHECK (dust_jacket IN (0, 1)),
    signed INTEGER NOT NULL DEFAULT 0 CHECK (signed IN (0, 1)),
    location TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
, gone_as TEXT NOT NULL DEFAULT '' CHECK (gone_as IN ('', 'lost', 'sold', 'donated', 'given', 'discarded')), gone_on TEXT NOT NULL DEFAULT '', gone_note TEXT NOT NULL DEFAULT '', seen_on TEXT NOT NULL DEFAULT '');
INSERT INTO copies_old (id, edition_id, section_id, acquired_as, acquired_on, acquired_from, price_cents, condition, dust_jacket, signed, location, notes, created_at, updated_at, gone_as, gone_on, gone_note, seen_on)
SELECT id, edition_id, section_id, COALESCE(acquired_as, ''), COALESCE(acquired_on, ''), COALESCE(acquired_from, ''), price_cents, COALESCE(condition, ''), dust_jacket, signed, COALESCE(location, ''), COALESCE(notes, ''), created_at, updated_at, COALESCE(gone_as, ''), COALESCE(gone_on, ''), COALESCE(gone_note, ''), COALESCE(seen_on, '')
FROM copies;
INSERT INTO rebuild_checks SELECT 'copies rows', (SELECT count(*) FROM copies) = (SELECT count(*) FROM copies_old);
DROP TABLE copies;
ALTER TABLE copies_old RENAME TO copies;
CREATE INDEX copies_edition_idx ON copies (edition_id);
-- +goose StatementBegin
CREATE TRIGGER copies_changed_delete AFTER DELETE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_insert AFTER INSERT ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_update AFTER UPDATE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd

CREATE TABLE enrichment_old (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_error TEXT NOT NULL DEFAULT '',
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
, fresh INTEGER NOT NULL DEFAULT 0 CHECK (fresh IN (0, 1)));
INSERT INTO enrichment_old (edition_id, status, attempts, next_at, last_error, queued_at, fresh)
SELECT edition_id, status, attempts, next_at, COALESCE(last_error, ''), queued_at, fresh
FROM enrichment;
INSERT INTO rebuild_checks SELECT 'enrichment rows', (SELECT count(*) FROM enrichment) = (SELECT count(*) FROM enrichment_old);
DROP TABLE enrichment;
ALTER TABLE enrichment_old RENAME TO enrichment;
CREATE INDEX enrichment_due_idx ON enrichment (status, next_at);
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_delete AFTER DELETE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_insert AFTER INSERT ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_update AFTER UPDATE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd

CREATE TABLE lookups_old (
    lookup_key TEXT NOT NULL,
    source TEXT NOT NULL,
    fetched_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    body BLOB NOT NULL,
    PRIMARY KEY (lookup_key, source)
);
INSERT INTO lookups_old (lookup_key, source, fetched_at, body)
SELECT lookup_key, source, fetched_at, body
FROM lookups;
INSERT INTO rebuild_checks SELECT 'lookups rows', (SELECT count(*) FROM lookups) = (SELECT count(*) FROM lookups_old);
DROP TABLE lookups;
ALTER TABLE lookups_old RENAME TO lookups;

CREATE TABLE shares_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT NOT NULL DEFAULT ''
, books TEXT NOT NULL DEFAULT 'all' CHECK (books IN ('all', 'some', 'none')), wishlist INTEGER NOT NULL DEFAULT 1 CHECK (wishlist IN (0, 1)), papers TEXT NOT NULL DEFAULT 'none' CHECK (papers IN ('all', 'some', 'none')));
INSERT INTO shares_old (id, name, token_hash, expires_at, created_at, last_used_at, books, wishlist, papers)
SELECT id, name, token_hash, COALESCE(expires_at, ''), created_at, COALESCE(last_used_at, ''), books, wishlist, papers
FROM shares;
INSERT INTO rebuild_checks SELECT 'shares rows', (SELECT count(*) FROM shares) = (SELECT count(*) FROM shares_old);
DROP TABLE shares;
ALTER TABLE shares_old RENAME TO shares;

CREATE TABLE borrowers_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
INSERT INTO borrowers_old (id, name, contact, notes, created_at)
SELECT id, name, COALESCE(contact, ''), COALESCE(notes, ''), created_at
FROM borrowers;
INSERT INTO rebuild_checks SELECT 'borrowers rows', (SELECT count(*) FROM borrowers) = (SELECT count(*) FROM borrowers_old);
DROP TABLE borrowers;
ALTER TABLE borrowers_old RENAME TO borrowers;

CREATE TABLE loans_old (
    id INTEGER PRIMARY KEY,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    borrower_id INTEGER NOT NULL REFERENCES borrowers (id) ON DELETE CASCADE,
    lent_on TEXT NOT NULL,
    due_on TEXT NOT NULL DEFAULT '',
    returned_on TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
INSERT INTO loans_old (id, copy_id, borrower_id, lent_on, due_on, returned_on, notes, created_at)
SELECT id, copy_id, borrower_id, lent_on, COALESCE(due_on, ''), COALESCE(returned_on, ''), COALESCE(notes, ''), created_at
FROM loans;
INSERT INTO rebuild_checks SELECT 'loans rows', (SELECT count(*) FROM loans) = (SELECT count(*) FROM loans_old);
DROP TABLE loans;
ALTER TABLE loans_old RENAME TO loans;
CREATE INDEX loans_borrower_idx ON loans (borrower_id);
CREATE INDEX loans_copy_idx ON loans (copy_id);
CREATE UNIQUE INDEX loans_out_idx ON loans (copy_id) WHERE returned_on = '';
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

CREATE TABLE courses_old (
    id INTEGER PRIMARY KEY,
    school TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT '' CHECK (level IN ('', 'high_school', 'undergraduate', 'graduate', 'professional', 'other')),
    department TEXT NOT NULL DEFAULT '',
    number TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    term TEXT NOT NULL DEFAULT '' CHECK (term IN ('', 'winter', 'spring', 'summer', 'fall')),
    year INTEGER CHECK (year BETWEEN 1900 AND 2100),
    instructor TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
INSERT INTO courses_old (id, school, level, department, number, title, term, year, instructor, notes, created_at, updated_at)
SELECT id, COALESCE(school, ''), COALESCE(level, ''), COALESCE(department, ''), COALESCE(number, ''), COALESCE(title, ''), COALESCE(term, ''), year, COALESCE(instructor, ''), COALESCE(notes, ''), created_at, updated_at
FROM courses;
INSERT INTO rebuild_checks SELECT 'courses rows', (SELECT count(*) FROM courses) = (SELECT count(*) FROM courses_old);
DROP TABLE courses;
ALTER TABLE courses_old RENAME TO courses;

CREATE TABLE label_queue_old (
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('barcode', 'spine')),
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (copy_id, kind)
);
INSERT INTO label_queue_old (copy_id, kind, queued_at)
SELECT copy_id, kind, queued_at
FROM label_queue;
INSERT INTO rebuild_checks SELECT 'label_queue rows', (SELECT count(*) FROM label_queue) = (SELECT count(*) FROM label_queue_old);
DROP TABLE label_queue;
ALTER TABLE label_queue_old RENAME TO label_queue;
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_delete AFTER DELETE ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', OLD.copy_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_insert AFTER INSERT ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', NEW.copy_id);
END;
-- +goose StatementEnd

CREATE TABLE document_files_old (
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
INSERT INTO document_files_old (id, document_id, name, stored, content_type, size, position, text, text_status, text_attempts, text_error, created_at)
SELECT id, document_id, name, stored, content_type, size, position, COALESCE(text, ''), text_status, text_attempts, COALESCE(text_error, ''), created_at
FROM document_files;
INSERT INTO rebuild_checks SELECT 'document_files rows', (SELECT count(*) FROM document_files) = (SELECT count(*) FROM document_files_old);
DROP TABLE document_files;
ALTER TABLE document_files_old RENAME TO document_files;
CREATE INDEX document_files_document_idx ON document_files (document_id, position);
CREATE INDEX document_files_text_idx ON document_files (text_status, id);
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_delete AFTER DELETE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_insert AFTER INSERT ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_update AFTER UPDATE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd

CREATE TABLE bookcases_old (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
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
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
, brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200), brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200), area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL);
INSERT INTO shelves_old (id, bookcase_id, position, width_mm, height_mm, depth_mm, starts_at, created_at, brace_height_mm, brace_width_mm, area_id)
SELECT id, bookcase_id, position, width_mm, height_mm, depth_mm, COALESCE(starts_at, ''), created_at, brace_height_mm, brace_width_mm, area_id
FROM shelves;
INSERT INTO rebuild_checks SELECT 'shelves rows', (SELECT count(*) FROM shelves) = (SELECT count(*) FROM shelves_old);
DROP TABLE shelves;
ALTER TABLE shelves_old RENAME TO shelves;
CREATE INDEX shelves_area_idx ON shelves (area_id);
CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

CREATE TABLE layout_settings_old (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    fill_percent INTEGER NOT NULL DEFAULT 80 CHECK (fill_percent BETWEEN 50 AND 100),
    applied_at TEXT NOT NULL DEFAULT ''
);
INSERT INTO layout_settings_old (id, fill_percent, applied_at)
SELECT id, fill_percent, COALESCE(applied_at, '')
FROM layout_settings;
INSERT INTO rebuild_checks SELECT 'layout_settings rows', (SELECT count(*) FROM layout_settings) = (SELECT count(*) FROM layout_settings_old);
DROP TABLE layout_settings;
ALTER TABLE layout_settings_old RENAME TO layout_settings;

CREATE TABLE scans_old (
    id INTEGER PRIMARY KEY,
    isbn13 TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'found', 'adding', 'added', 'owned', 'not_found', 'failed')),
    proposal TEXT NOT NULL DEFAULT '',
    edition_id INTEGER REFERENCES editions (id) ON DELETE SET NULL,
    error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    reviewed INTEGER NOT NULL DEFAULT 0 CHECK (reviewed IN (0, 1)),
    scanned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    done_at TEXT
, want INTEGER NOT NULL DEFAULT 0 CHECK (want IN (0, 1)));
INSERT INTO scans_old (id, isbn13, section, acquired_as, status, proposal, edition_id, error, attempts, reviewed, scanned_at, done_at, want)
SELECT id, isbn13, COALESCE(section, ''), COALESCE(acquired_as, ''), status, COALESCE(proposal, ''), edition_id, COALESCE(error, ''), attempts, reviewed, scanned_at, done_at, want
FROM scans;
INSERT INTO rebuild_checks SELECT 'scans rows', (SELECT count(*) FROM scans) = (SELECT count(*) FROM scans_old);
DROP TABLE scans;
ALTER TABLE scans_old RENAME TO scans;
CREATE INDEX scans_open_idx ON scans (reviewed, id);
CREATE INDEX scans_status_idx ON scans (status, id);
-- +goose StatementBegin
CREATE TRIGGER scans_changed_delete AFTER DELETE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_insert AFTER INSERT ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_update AFTER UPDATE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd

CREATE TABLE readings_old (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    started_on TEXT NOT NULL DEFAULT '',
    finished_on TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
INSERT INTO readings_old (id, work_id, started_on, finished_on, created_at)
SELECT id, work_id, COALESCE(started_on, ''), COALESCE(finished_on, ''), created_at
FROM readings;
INSERT INTO rebuild_checks SELECT 'readings rows', (SELECT count(*) FROM readings) = (SELECT count(*) FROM readings_old);
DROP TABLE readings;
ALTER TABLE readings_old RENAME TO readings;
CREATE INDEX readings_work_idx ON readings (work_id, finished_on);
-- +goose StatementBegin
CREATE TRIGGER readings_changed_delete AFTER DELETE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = OLD.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_insert AFTER INSERT ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_update AFTER UPDATE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd

CREATE TABLE calendar_links_old (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT NOT NULL DEFAULT ''
);
INSERT INTO calendar_links_old (id, token_hash, created_at, last_used_at)
SELECT id, token_hash, created_at, COALESCE(last_used_at, '')
FROM calendar_links;
INSERT INTO rebuild_checks SELECT 'calendar_links rows', (SELECT count(*) FROM calendar_links) = (SELECT count(*) FROM calendar_links_old);
DROP TABLE calendar_links;
ALTER TABLE calendar_links_old RENAME TO calendar_links;

CREATE TABLE changes_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('book', 'scan', 'document', 'lookup', 'loan', 'label')),
    ref INTEGER NOT NULL,
    -- When, so a new stream can start a few seconds back (events.Feed.Lookback).
    at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
INSERT INTO changes_old (id, type, ref, at)
SELECT id, type, ref, at
FROM changes;
INSERT INTO rebuild_checks SELECT 'changes rows', (SELECT count(*) FROM changes) = (SELECT count(*) FROM changes_old);
DROP TABLE changes;
ALTER TABLE changes_old RENAME TO changes;

CREATE TABLE documents_old (
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
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    subjects TEXT NOT NULL DEFAULT '[]',
    abstract TEXT NOT NULL DEFAULT '',
    editors TEXT NOT NULL DEFAULT '[]'
);
INSERT INTO documents_old (id, kind, title, authors, container, volume, issue, pages, published, publisher, doi, url, course_id, notes, created_at, updated_at, subjects, abstract, editors)
SELECT id, kind, title, authors, COALESCE(container, ''), COALESCE(volume, ''), COALESCE(issue, ''), COALESCE(pages, ''), COALESCE(published, ''), COALESCE(publisher, ''), COALESCE(doi, ''), COALESCE(url, ''), course_id, COALESCE(notes, ''), created_at, updated_at, subjects, COALESCE(abstract, ''), editors
FROM documents;
INSERT INTO rebuild_checks SELECT 'documents rows', (SELECT count(*) FROM documents) = (SELECT count(*) FROM documents_old);
DROP TABLE documents;
ALTER TABLE documents_old RENAME TO documents;
CREATE INDEX documents_course_idx ON documents (course_id);
CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';
-- +goose StatementBegin
CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;
-- +goose StatementEnd
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

INSERT INTO sqlite_sequence (name, seq)
SELECT k.name, k.seq FROM temp.kept_sequence k
WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence s WHERE s.name = k.name);
UPDATE sqlite_sequence
SET seq = max(seq, (SELECT k.seq FROM temp.kept_sequence k WHERE k.name = sqlite_sequence.name))
WHERE name IN (SELECT name FROM temp.kept_sequence);

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
DROP TABLE temp.kept_sequence;
PRAGMA user_version = 37;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;
