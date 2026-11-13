-- name: CreateArea :one
INSERT INTO areas (name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm)
VALUES (
    sqlc.arg(name), (SELECT COALESCE(max(position), 0) + 1 FROM areas), sqlc.arg(dewey_from),
    sqlc.arg(dewey_to), sqlc.arg(subject), sqlc.arg(min_height_mm), sqlc.arg(max_height_mm)
)
RETURNING id;

-- name: UpdateArea :execrows
UPDATE areas
SET name = ?, dewey_from = ?, dewey_to = ?, subject = ?, min_height_mm = ?, max_height_mm = ?
WHERE id = ?;

-- name: SetAreaPosition :exec
UPDATE areas SET position = ? WHERE id = ?;

-- name: DeleteArea :execrows
DELETE FROM areas WHERE id = ?;

-- name: ListAreas :many
-- The areas in the order a book tries them.
SELECT id, name, position, dewey_from, dewey_to, subject, min_height_mm, max_height_mm
FROM areas ORDER BY position, id;

-- name: ListAreaRules :many
-- Every area's lists, a row each: a category by its name, an author or series by number and
-- name, and the order by rank and key.
SELECT area_id, CAST('category' AS TEXT) AS kind, CAST(0 AS INTEGER) AS ref, category AS name
FROM area_categories
UNION ALL
SELECT aa.area_id, 'author', p.id, p.name FROM area_authors aa JOIN people p ON p.id = aa.person_id
UNION ALL
SELECT asr.area_id, 'series', sr.id, sr.name FROM area_series asr JOIN series sr ON sr.id = asr.series_id
UNION ALL
SELECT area_id, 'order', rank, key FROM area_order
ORDER BY 1, 2, 3;

-- name: ClearAreaCategories :exec
DELETE FROM area_categories WHERE area_id = ?;

-- name: ClearAreaAuthors :exec
DELETE FROM area_authors WHERE area_id = ?;

-- name: ClearAreaSeries :exec
DELETE FROM area_series WHERE area_id = ?;

-- name: ClearAreaOrder :exec
DELETE FROM area_order WHERE area_id = ?;

-- name: AddAreaCategory :exec
INSERT INTO area_categories (area_id, category) VALUES (?, ?);

-- name: AddAreaAuthor :exec
INSERT INTO area_authors (area_id, person_id) VALUES (?, ?);

-- name: AddAreaSeries :exec
INSERT INTO area_series (area_id, series_id) VALUES (?, ?);

-- name: AddAreaOrder :exec
INSERT INTO area_order (area_id, rank, key) VALUES (?, ?, ?);

-- name: ClearAreaShelves :exec
UPDATE shelves SET area_id = NULL WHERE area_id = ?;

-- name: SetShelfArea :execrows
UPDATE shelves SET area_id = ? WHERE id = ?;

-- name: GetPersonByName :one
SELECT id, name FROM people WHERE name = ? COLLATE NOCASE;

-- name: ListAuthorNames :many
-- Everyone the library has a book by, for the names an area's rules offer.
SELECT DISTINCT p.name
FROM people p JOIN edition_contributors ec ON ec.person_id = p.id AND ec.role = 'author'
ORDER BY p.name COLLATE NOCASE;
