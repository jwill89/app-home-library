-- +goose Up

-- Barcodes scanned in Add a shelf. Each is looked up and its book added in the background,
-- and the open ones are the list the owner reviews afterwards.
CREATE TABLE scans (
    id INTEGER PRIMARY KEY,
    isbn13 TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'added', 'owned', 'not_found', 'failed')),
    edition_id INTEGER REFERENCES editions (id) ON DELETE SET NULL,
    error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    reviewed INTEGER NOT NULL DEFAULT 0 CHECK (reviewed IN (0, 1)),
    scanned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    done_at TEXT
);

CREATE INDEX scans_open_idx ON scans (reviewed, id);
CREATE INDEX scans_status_idx ON scans (status, id);

-- A scan as the review list shows it: with the book it found or added, when it has one.
CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.error, s.scanned_at, s.reviewed,
    CAST(COALESCE(s.edition_id, 0) AS INTEGER) AS edition_id,
    CAST(COALESCE(e.title, '') AS TEXT) AS title,
    CAST(COALESCE(e.cover, '') AS TEXT) AS cover,
    CAST(COALESCE((
        SELECT group_concat(name, ', ') FROM (
            SELECT p.name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
            WHERE ec.edition_id = s.edition_id AND ec.role = 'author' ORDER BY ec.position
        )
    ), '') AS TEXT) AS authors
FROM scans s
LEFT JOIN editions e ON e.id = s.edition_id;

-- +goose Down
DROP VIEW scan_rows;
DROP TABLE scans;
