-- +goose Up

-- A scanned book waits for the owner's word. Each scan is looked up in the background and held
-- as found, with what the sources said; the owner adds the found ones, one or all at once, and
-- they are added in the background in turn. The section and how it was bought are the owner's
-- choice at that moment, not at the scan.
DROP VIEW scan_rows;

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
    scanned_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    done_at TEXT
);

INSERT INTO scans_new (id, isbn13, section, acquired_as, status, edition_id, error, attempts, reviewed, scanned_at, done_at)
SELECT id, isbn13, section, acquired_as, status, edition_id, error, attempts, reviewed, scanned_at, done_at FROM scans;

DROP TABLE scans;
ALTER TABLE scans_new RENAME TO scans;

CREATE INDEX scans_open_idx ON scans (reviewed, id);
CREATE INDEX scans_status_idx ON scans (status, id);

-- A scan as the list shows it: with the book it added or found in the library, when it has
-- one, and otherwise what the sources said it is.
CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.proposal, s.error, s.scanned_at, s.reviewed,
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

CREATE TABLE scans_old (
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

INSERT INTO scans_old (id, isbn13, section, acquired_as, status, edition_id, error, attempts, reviewed, scanned_at, done_at)
SELECT id, isbn13, section, acquired_as,
    CASE status WHEN 'found' THEN 'not_found' WHEN 'adding' THEN 'pending' ELSE status END,
    edition_id, error, attempts, reviewed, scanned_at, done_at
FROM scans;

DROP TABLE scans;
ALTER TABLE scans_old RENAME TO scans;

CREATE INDEX scans_open_idx ON scans (reviewed, id);
CREATE INDEX scans_status_idx ON scans (status, id);

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
