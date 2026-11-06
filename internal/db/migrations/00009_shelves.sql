-- +goose Up

-- The bookcases, in the order they are walked past, and their shelves from the top down.
-- A shelf's width is what is inside its sides, its height the clear space above it, its depth
-- front to back. An oversize shelf takes the books too tall for the shelf their call number
-- would put them on.
CREATE TABLE bookcases (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE shelves (
    id INTEGER PRIMARY KEY,
    bookcase_id INTEGER NOT NULL REFERENCES bookcases (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 100 AND 5000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 50 AND 1000),
    depth_mm INTEGER NOT NULL CHECK (depth_mm BETWEEN 50 AND 1000),
    oversize INTEGER NOT NULL DEFAULT 0 CHECK (oversize IN (0, 1)),
    -- Where the shelf begins in shelf order, as the layout in use put it: the first book's
    -- shelving, as JSON. Empty while no layout is in use, or for a shelf added since.
    starts_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

-- How full the planner fills a shelf, leaving the rest for books to come, and when the layout
-- in use was put to use.
CREATE TABLE layout_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    fill_percent INTEGER NOT NULL DEFAULT 80 CHECK (fill_percent BETWEEN 50 AND 100),
    applied_at TEXT NOT NULL DEFAULT ''
);

INSERT INTO layout_settings (id) VALUES (1);

-- +goose Down
DROP TABLE layout_settings;
DROP TABLE shelves;
DROP TABLE bookcases;
