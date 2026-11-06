-- +goose Up

-- A copy that has gone (lost, sold, donated, given away, discarded) keeps its record with the
-- day and a note, and leaves the shelves: the shelving view, which the labels and the planner
-- read, holds only the copies still here. A shelf check records the day a copy was last seen.
ALTER TABLE copies ADD COLUMN gone_as TEXT NOT NULL DEFAULT '' CHECK (gone_as IN ('', 'lost', 'sold', 'donated', 'given', 'discarded'));
ALTER TABLE copies ADD COLUMN gone_on TEXT NOT NULL DEFAULT '';
ALTER TABLE copies ADD COLUMN gone_note TEXT NOT NULL DEFAULT '';
ALTER TABLE copies ADD COLUMN seen_on TEXT NOT NULL DEFAULT '';

DROP VIEW copy_shelving;
CREATE VIEW copy_shelving AS
SELECT
    c.id AS copy_id,
    c.edition_id,
    CAST(COALESCE(sec.name, '') AS TEXT) AS section,
    CAST(COALESCE(sec.shelving, 'dewey') AS TEXT) AS shelving,
    CAST(COALESCE(sec.label_prefix, '') AS TEXT) AS label_prefix,
    e.ddc,
    e.title,
    CAST(COALESCE(s.name, '') AS TEXT) AS series,
    w.series_number,
    CAST(COALESCE((
        SELECT p.sort_name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
        WHERE ec.edition_id = c.edition_id AND ec.role = 'author'
        ORDER BY ec.position
        LIMIT 1
    ), '') AS TEXT) AS author_sort
FROM copies c
JOIN editions e ON e.id = c.edition_id
JOIN works w ON w.id = e.work_id
LEFT JOIN series s ON s.id = w.series_id
LEFT JOIN sections sec ON sec.id = c.section_id
WHERE c.gone_as = '';

-- +goose Down
DROP VIEW copy_shelving;
CREATE VIEW copy_shelving AS
SELECT
    c.id AS copy_id,
    c.edition_id,
    CAST(COALESCE(sec.name, '') AS TEXT) AS section,
    CAST(COALESCE(sec.shelving, 'dewey') AS TEXT) AS shelving,
    CAST(COALESCE(sec.label_prefix, '') AS TEXT) AS label_prefix,
    e.ddc,
    e.title,
    CAST(COALESCE(s.name, '') AS TEXT) AS series,
    w.series_number,
    CAST(COALESCE((
        SELECT p.sort_name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
        WHERE ec.edition_id = c.edition_id AND ec.role = 'author'
        ORDER BY ec.position
        LIMIT 1
    ), '') AS TEXT) AS author_sort
FROM copies c
JOIN editions e ON e.id = c.edition_id
JOIN works w ON w.id = e.work_id
LEFT JOIN series s ON s.id = w.series_id
LEFT JOIN sections sec ON sec.id = c.section_id;
ALTER TABLE copies DROP COLUMN seen_on;
ALTER TABLE copies DROP COLUMN gone_note;
ALTER TABLE copies DROP COLUMN gone_on;
ALTER TABLE copies DROP COLUMN gone_as;
