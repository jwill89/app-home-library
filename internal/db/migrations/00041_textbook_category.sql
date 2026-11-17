-- +goose NO TRANSACTION
-- +goose Up

-- Textbooks are a category of their own (2026-11-17): set by the owner, or worked out for a
-- book with a copy on a course. A CHECK can't change in place, so the three tables that name
-- the categories are rebuilt as 00038's were, each as it is but for the new one. A copy put on
-- a course, or taken off, now tells the change feed of its book, whose category it can change.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);

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
    category TEXT NOT NULL DEFAULT ''
        CHECK (category IN ('', 'fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized'))
);
INSERT INTO works_new (id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at, category)
SELECT id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at, category
FROM works;
INSERT INTO rebuild_checks SELECT 'works rows', (SELECT count(*) FROM works) = (SELECT count(*) FROM works_new);
DROP TABLE works;
ALTER TABLE works_new RENAME TO works;
CREATE INDEX works_series_idx ON works (series_id);
-- +goose StatementBegin
CREATE TRIGGER works_changed AFTER UPDATE ON works BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.id;
END;
-- +goose StatementEnd

CREATE TABLE area_categories_new (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (area_id, category)
) WITHOUT ROWID;
INSERT INTO area_categories_new (area_id, category) SELECT area_id, category FROM area_categories;
INSERT INTO rebuild_checks SELECT 'area_categories rows', (SELECT count(*) FROM area_categories) = (SELECT count(*) FROM area_categories_new);
DROP TABLE area_categories;
ALTER TABLE area_categories_new RENAME TO area_categories;

CREATE TABLE share_categories_new (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (share_id, category)
) WITHOUT ROWID;
INSERT INTO share_categories_new (share_id, category) SELECT share_id, category FROM share_categories;
INSERT INTO rebuild_checks SELECT 'share_categories rows', (SELECT count(*) FROM share_categories) = (SELECT count(*) FROM share_categories_new);
DROP TABLE share_categories;
ALTER TABLE share_categories_new RENAME TO share_categories;

-- Every edition of the work changes, since the category is the work's.
-- +goose StatementBegin
CREATE TRIGGER course_copies_changed_insert AFTER INSERT ON course_copies BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e
    WHERE e.work_id = (SELECT ce.work_id FROM copies c JOIN editions ce ON ce.id = c.edition_id WHERE c.id = NEW.copy_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER course_copies_changed_delete AFTER DELETE ON course_copies BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e
    WHERE e.work_id = (SELECT ce.work_id FROM copies c JOIN editions ce ON ce.id = c.edition_id WHERE c.id = OLD.copy_id);
END;
-- +goose StatementEnd

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
PRAGMA user_version = 41;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;

-- +goose Down
-- Each table as it was, without textbooks: a book set as one is worked out again, and an area
-- or a share link that took them no longer does.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);

DROP TRIGGER course_copies_changed_insert;
DROP TRIGGER course_copies_changed_delete;

CREATE TABLE works_old (
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
INSERT INTO works_old (id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at, category)
SELECT id, title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, created_at, updated_at,
    CASE category WHEN 'textbook' THEN '' ELSE category END
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

CREATE TABLE area_categories_old (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (area_id, category)
) WITHOUT ROWID;
INSERT INTO area_categories_old (area_id, category)
SELECT area_id, category FROM area_categories WHERE category <> 'textbook';
DROP TABLE area_categories;
ALTER TABLE area_categories_old RENAME TO area_categories;

CREATE TABLE share_categories_old (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (share_id, category)
) WITHOUT ROWID;
INSERT INTO share_categories_old (share_id, category)
SELECT share_id, category FROM share_categories WHERE category <> 'textbook';
DROP TABLE share_categories;
ALTER TABLE share_categories_old RENAME TO share_categories;

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
PRAGMA user_version = 40;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;
