-- +goose Up

-- Areas of shelves, each with rules for which books it takes and in what order (2026-11-13). A
-- book goes to the first area, in the owner's order, whose rules take it, and an area fills its
-- shelves in walking order. An area for tall books, put first, does what the oversize shelves
-- did. A layout is no longer proposed unasked, and none was in use, so none carries over.
CREATE TABLE areas (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    position INTEGER NOT NULL,
    dewey_from TEXT NOT NULL DEFAULT '',
    dewey_to TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    min_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (min_height_mm BETWEEN 0 AND 1000),
    max_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (max_height_mm BETWEEN 0 AND 1000)
);

CREATE TABLE area_categories (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (area_id, category)
) WITHOUT ROWID;

-- An author or series removed leaves every area's rules.
CREATE TABLE area_authors (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
) WITHOUT ROWID;
CREATE INDEX area_authors_person_idx ON area_authors (person_id);

CREATE TABLE area_series (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    series_id INTEGER NOT NULL REFERENCES series (id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, series_id)
) WITHOUT ROWID;
CREATE INDEX area_series_series_idx ON area_series (series_id);

-- What an area orders its books by, first key first.
CREATE TABLE area_order (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    rank INTEGER NOT NULL CHECK (rank >= 1),
    key TEXT NOT NULL CHECK (key IN ('subject', 'author', 'series', 'title', 'published', 'height')),
    PRIMARY KEY (area_id, rank)
) WITHOUT ROWID;

ALTER TABLE shelves ADD COLUMN area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL;
CREATE INDEX shelves_area_idx ON shelves (area_id);
ALTER TABLE shelves DROP COLUMN oversize;
UPDATE shelves SET starts_at = '';
UPDATE layout_settings SET applied_at = '';

-- +goose Down
-- The shelves' area cannot be dropped from a column a foreign key names: the table is rebuilt.
CREATE TABLE shelves_before (
    id INTEGER PRIMARY KEY,
    bookcase_id INTEGER NOT NULL REFERENCES bookcases (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 100 AND 5000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 50 AND 1000),
    depth_mm INTEGER NOT NULL CHECK (depth_mm BETWEEN 50 AND 1000),
    oversize INTEGER NOT NULL DEFAULT 0 CHECK (oversize IN (0, 1)),
    starts_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200),
    brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200)
);
INSERT INTO shelves_before (id, bookcase_id, position, width_mm, height_mm, depth_mm, created_at, brace_height_mm, brace_width_mm)
SELECT id, bookcase_id, position, width_mm, height_mm, depth_mm, created_at, brace_height_mm, brace_width_mm FROM shelves;
DROP TABLE shelves;
ALTER TABLE shelves_before RENAME TO shelves;
CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);
DROP TABLE area_order;
DROP TABLE area_series;
DROP TABLE area_authors;
DROP TABLE area_categories;
DROP TABLE areas;
