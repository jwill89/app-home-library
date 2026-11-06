-- +goose Up

-- The wishlist: an edition the owner wants and has no copy of, since the day it went on. Its
-- first copy takes it off. A scan of a found book may go on the wishlist instead of the shelf.
ALTER TABLE editions ADD COLUMN wanted_on TEXT NOT NULL DEFAULT '';
ALTER TABLE scans ADD COLUMN want INTEGER NOT NULL DEFAULT 0 CHECK (want IN (0, 1));

DROP VIEW scan_rows;
CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.proposal, s.error, s.scanned_at, s.reviewed,
    CAST(COALESCE(s.edition_id, 0) AS INTEGER) AS edition_id,
    CAST(COALESCE(e.title, '') AS TEXT) AS title,
    CAST(COALESCE(e.cover, '') AS TEXT) AS cover,
    CAST(COALESCE(e.wanted_on, '') AS TEXT) AS wanted_on,
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
ALTER TABLE scans DROP COLUMN want;
ALTER TABLE editions DROP COLUMN wanted_on;
