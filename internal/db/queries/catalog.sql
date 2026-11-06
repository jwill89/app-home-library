-- name: UpsertPerson :one
INSERT INTO people (name, sort_name) VALUES (?, ?)
ON CONFLICT (name) DO UPDATE SET name = people.name
RETURNING id;

-- name: UpsertSeries :one
INSERT INTO series (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = series.name
RETURNING id;

-- name: UpsertSection :one
INSERT INTO sections (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = sections.name
RETURNING id;

-- name: UpsertGenre :one
INSERT INTO genres (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = genres.name
RETURNING id;

-- name: ListSections :many
SELECT name FROM sections ORDER BY name;

-- name: ListGenres :many
SELECT name FROM genres ORDER BY name;

-- name: ListSeriesNames :many
-- The names the form offers: not one put aside as no series.
SELECT name FROM series WHERE dismissed_on = '' ORDER BY name;

-- name: CreateWork :one
INSERT INTO works (title, series_id, series_number, read_status, current_page, started_on, finished_on, rating, notes, category)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateWork :exec
UPDATE works
SET title = ?, series_id = ?, series_number = ?, read_status = ?, current_page = ?,
    started_on = ?, finished_on = ?, rating = ?, notes = ?, category = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: GetWork :one
SELECT w.*, COALESCE(s.name, '') AS series_name
FROM works w LEFT JOIN series s ON s.id = w.series_id
WHERE w.id = ?;

-- name: DeleteWorkWithoutEditions :exec
DELETE FROM works
WHERE works.id = sqlc.arg(work_id) AND NOT EXISTS (SELECT 1 FROM editions e WHERE e.work_id = sqlc.arg(work_id));

-- name: ClearWorkGenres :exec
DELETE FROM work_genres WHERE work_id = ?;

-- name: AddWorkGenre :exec
INSERT OR IGNORE INTO work_genres (work_id, genre_id) VALUES (?, ?);

-- name: ListWorkGenres :many
SELECT g.name FROM work_genres wg JOIN genres g ON g.id = wg.genre_id WHERE wg.work_id = ? ORDER BY g.name;

-- name: CreateEdition :one
INSERT INTO editions (
    work_id, isbn13, isbn10, lccn, title, subtitle, publisher, publish_place, published,
    edition_statement, format, pages, height_mm, width_mm, thickness_mm, language, ddc, lcc,
    subjects, description, sources
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateEdition :exec
UPDATE editions
SET isbn13 = ?, isbn10 = ?, lccn = ?, title = ?, subtitle = ?, publisher = ?, publish_place = ?,
    published = ?, edition_statement = ?, format = ?, pages = ?, height_mm = ?, width_mm = ?,
    thickness_mm = ?, language = ?, ddc = ?, lcc = ?, subjects = ?, description = ?, sources = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: SetEditionCover :exec
UPDATE editions
SET cover = ?, sources = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: GetEdition :one
SELECT * FROM editions WHERE id = ?;

-- name: GetEditionByISBN :one
SELECT * FROM editions WHERE isbn13 = ?;

-- name: DeleteEdition :exec
DELETE FROM editions WHERE id = ?;

-- name: ClearEditionContributors :exec
DELETE FROM edition_contributors WHERE edition_id = ?;

-- name: AddEditionContributor :exec
INSERT INTO edition_contributors (edition_id, person_id, role, position) VALUES (?, ?, ?, ?);

-- name: ListEditionContributors :many
SELECT p.name, ec.role
FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
WHERE ec.edition_id = ?
ORDER BY CASE ec.role WHEN 'author' THEN 0 WHEN 'editor' THEN 1 ELSE 2 END, ec.role, ec.position;

-- name: SeriesDDC :one
-- The number most of a series already has, for a volume no source has catalogued yet.
SELECT e.ddc, count(*) AS n
FROM editions e JOIN works w ON w.id = e.work_id JOIN series s ON s.id = w.series_id
WHERE s.name = ? AND e.ddc <> ''
GROUP BY e.ddc
ORDER BY n DESC
LIMIT 1;

-- name: SetCopyGone :execrows
UPDATE copies SET gone_as = ?, gone_on = ?, gone_note = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: SetCopySeen :execrows
UPDATE copies SET seen_on = ? WHERE id = ?;

-- name: SetEditionWanted :exec
-- On the wishlist since the day given, or off it with an empty day.
UPDATE editions SET wanted_on = ? WHERE id = ?;

-- name: CreateCopy :one
INSERT INTO copies (edition_id, section_id, acquired_as, acquired_on, acquired_from, price_cents, condition, dust_jacket, signed, location, notes)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateCopy :exec
UPDATE copies
SET section_id = ?, acquired_as = ?, acquired_on = ?, acquired_from = ?, price_cents = ?,
    condition = ?, dust_jacket = ?, signed = ?, location = ?, notes = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: GetCopy :one
SELECT c.*, COALESCE(s.name, '') AS section_name
FROM copies c LEFT JOIN sections s ON s.id = c.section_id
WHERE c.id = ?;

-- name: DeleteCopy :exec
DELETE FROM copies WHERE id = ?;

-- name: ListEditionCopies :many
SELECT c.*, COALESCE(s.name, '') AS section_name
FROM copies c LEFT JOIN sections s ON s.id = c.section_id
WHERE c.edition_id = ?
ORDER BY c.id;

-- name: ListBooks :many
-- The whole library, one row an edition. The list is searched, filtered, and sorted in the
-- browser: a household's books are a few thousand rows at most.
SELECT
    e.id, e.title, e.subtitle, e.isbn13, e.format, e.ddc, e.lcc, e.published, e.publisher, e.cover,
    e.subjects, e.created_at, e.wanted_on, w.id AS work_id, w.read_status, w.series_number, w.category,
    CAST(COALESCE(w.series_id, 0) AS INTEGER) AS series_id, COALESCE(s.name, '') AS series_name,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors,
    CAST(COALESCE((
        SELECT group_concat(p.name, ', ') FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
        WHERE ec.edition_id = e.id AND ec.role <> 'author'
    ), '') AS TEXT) AS credits,
    CAST(COALESCE((
        SELECT group_concat(g.name, ', ') FROM work_genres wg JOIN genres g ON g.id = wg.genre_id
        WHERE wg.work_id = w.id
    ), '') AS TEXT) AS genres,
    CAST(COALESCE((
        SELECT group_concat(DISTINCT sec.name) FROM copies c JOIN sections sec ON sec.id = c.section_id
        WHERE c.edition_id = e.id
    ), '') AS TEXT) AS sections,
    (SELECT count(*) FROM copies c WHERE c.edition_id = e.id AND c.gone_as = '') AS copy_count,
    CAST(COALESCE((SELECT min(c.id) FROM copies c WHERE c.edition_id = e.id AND c.gone_as = ''), 0) AS INTEGER) AS first_copy_id
FROM editions e
JOIN works w ON w.id = e.work_id
LEFT JOIN series s ON s.id = w.series_id
LEFT JOIN edition_authors ea ON ea.edition_id = e.id
ORDER BY e.id;

-- name: GetImportRef :one
SELECT copy_id FROM import_refs WHERE source = ? AND external_id = ?;

-- name: CreateImportRef :exec
INSERT INTO import_refs (source, external_id, copy_id, link) VALUES (?, ?, ?, ?)
ON CONFLICT (source, external_id) DO NOTHING;

-- name: ListEditionLinks :many
-- Where the edition's copies are in the apps they were imported from, one address an app.
SELECT r.source, CAST(min(r.link) AS TEXT) AS link
FROM import_refs r JOIN copies c ON c.id = r.copy_id
WHERE c.edition_id = ? AND r.link <> ''
GROUP BY r.source
ORDER BY r.source;

-- name: ListBookGaps :many
-- What each book in the house has and lacks, for the page of missing details. A wanted book is
-- the wishlist's.
SELECT
    e.id, e.title, e.isbn13, e.cover, e.ddc, e.format, e.publisher, e.published,
    CAST(COALESCE(e.height_mm, 0) AS INTEGER) AS height_mm,
    CAST(COALESCE(e.thickness_mm, 0) AS INTEGER) AS thickness_mm,
    CAST(COALESCE(e.pages, 0) AS INTEGER) AS pages,
    CAST(length(e.description) AS INTEGER) AS description_length,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM editions e
LEFT JOIN edition_authors ea ON ea.edition_id = e.id
WHERE e.wanted_on = ''
ORDER BY e.title COLLATE NOCASE, e.id;
