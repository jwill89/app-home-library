-- +goose Up

-- How a section's copies are shelved, which is what their call numbers say: by Dewey number,
-- by author under a prefix (FIC MCD), or by series under a prefix (MANGA ONE v.12).
ALTER TABLE sections ADD COLUMN shelving TEXT NOT NULL DEFAULT 'dewey' CHECK (shelving IN ('dewey', 'author', 'series'));
ALTER TABLE sections ADD COLUMN label_prefix TEXT NOT NULL DEFAULT '';

UPDATE sections SET shelving = 'author', label_prefix = 'FIC' WHERE name = 'Fiction';
UPDATE sections SET shelving = 'series', label_prefix = 'MANGA' WHERE name = 'Manga and Light Novels';

-- What a copy's call number is made from, in one place for the book's page and the labels.
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

-- Labels waiting to be printed, a barcode and a spine label for each copy. They wait until a
-- sheet is full, since a label sheet half used should not go through a laser printer again.
CREATE TABLE label_queue (
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('barcode', 'spine')),
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (copy_id, kind)
);

-- Every copy already in the library has yet to be labelled.
INSERT INTO label_queue (copy_id, kind) SELECT id, 'barcode' FROM copies;
INSERT INTO label_queue (copy_id, kind) SELECT id, 'spine' FROM copies;

-- How far this printer's labels land from where they should, in millimetres: right and down
-- are positive.
CREATE TABLE label_offsets (
    kind TEXT PRIMARY KEY CHECK (kind IN ('barcode', 'spine')),
    x_mm INTEGER NOT NULL DEFAULT 0 CHECK (x_mm BETWEEN -10 AND 10),
    y_mm INTEGER NOT NULL DEFAULT 0 CHECK (y_mm BETWEEN -10 AND 10)
);

INSERT INTO label_offsets (kind) VALUES ('barcode'), ('spine');

-- +goose Down
DROP TABLE label_offsets;
DROP TABLE label_queue;
DROP VIEW copy_shelving;
ALTER TABLE sections DROP COLUMN label_prefix;
ALTER TABLE sections DROP COLUMN shelving;
