-- name: CreateBookcase :one
INSERT INTO bookcases (name, position)
VALUES (?, (SELECT COALESCE(max(position), 0) + 1 FROM bookcases))
RETURNING id;

-- name: RenameBookcase :execrows
UPDATE bookcases SET name = ? WHERE id = ?;

-- name: SetBookcasePosition :exec
UPDATE bookcases SET position = ? WHERE id = ?;

-- name: DeleteBookcase :execrows
DELETE FROM bookcases WHERE id = ?;

-- name: ListBookcases :many
SELECT * FROM bookcases ORDER BY position, id;

-- name: CreateShelf :one
INSERT INTO shelves (bookcase_id, position, width_mm, height_mm, depth_mm, brace_height_mm, brace_width_mm)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: NextShelfPosition :one
SELECT CAST(COALESCE(max(position), 0) + 1 AS INTEGER) FROM shelves WHERE bookcase_id = ?;

-- name: UpdateShelf :execrows
UPDATE shelves
SET width_mm = ?, height_mm = ?, depth_mm = ?, brace_height_mm = ?, brace_width_mm = ?
WHERE id = ?;

-- name: GetShelf :one
SELECT * FROM shelves WHERE id = ?;

-- name: DeleteShelf :exec
DELETE FROM shelves WHERE id = ?;

-- name: RenumberShelf :exec
UPDATE shelves SET position = ? WHERE id = ?;

-- name: ListShelves :many
-- Every shelf in walking order: bookcase by bookcase, each from the top down, with its area.
SELECT
    s.id, s.bookcase_id, s.position, s.width_mm, s.height_mm, s.depth_mm, s.brace_height_mm,
    s.brace_width_mm, s.starts_at, CAST(COALESCE(s.area_id, 0) AS INTEGER) AS area_id,
    CAST(COALESCE(a.name, '') AS TEXT) AS area, b.name AS bookcase
FROM shelves s
JOIN bookcases b ON b.id = s.bookcase_id
LEFT JOIN areas a ON a.id = s.area_id
ORDER BY b.position, b.id, s.position, s.id;

-- name: SetShelfStart :exec
UPDATE shelves SET starts_at = ? WHERE id = ?;

-- name: ClearShelfStarts :exec
UPDATE shelves SET starts_at = '';

-- name: GetLayoutSettings :one
SELECT * FROM layout_settings WHERE id = 1;

-- name: SetFillPercent :exec
UPDATE layout_settings SET fill_percent = ? WHERE id = 1;

-- name: SetLayoutApplied :exec
UPDATE layout_settings SET applied_at = ? WHERE id = 1;

-- name: ListShelfCopies :many
-- Every copy with what places it: its shelving, its size, and what an area's rules ask of it.
SELECT
    cs.*, e.format, e.cover, e.publisher, e.subjects, e.lcc, e.published,
    w.category, CAST(COALESCE(w.series_id, 0) AS INTEGER) AS series_id,
    CAST(COALESCE(e.pages, 0) AS INTEGER) AS pages,
    CAST(COALESCE(e.height_mm, 0) AS INTEGER) AS height_mm,
    CAST(COALESCE(e.thickness_mm, 0) AS INTEGER) AS thickness_mm,
    CAST(EXISTS (SELECT 1 FROM loans l WHERE l.copy_id = cs.copy_id AND l.returned_on = '') AS INTEGER) AS on_loan
FROM copy_shelving cs
JOIN editions e ON e.id = cs.edition_id
JOIN works w ON w.id = e.work_id;

-- name: ListEditionAuthorIDs :many
-- Who wrote each edition, for the areas that take an author's books.
SELECT edition_id, person_id FROM edition_contributors WHERE role = 'author';
