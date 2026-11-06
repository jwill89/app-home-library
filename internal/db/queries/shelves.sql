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
INSERT INTO shelves (bookcase_id, position, width_mm, height_mm, depth_mm, oversize, brace_height_mm, brace_width_mm)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: NextShelfPosition :one
SELECT CAST(COALESCE(max(position), 0) + 1 AS INTEGER) FROM shelves WHERE bookcase_id = ?;

-- name: UpdateShelf :execrows
UPDATE shelves
SET width_mm = ?, height_mm = ?, depth_mm = ?, oversize = ?, brace_height_mm = ?, brace_width_mm = ?
WHERE id = ?;

-- name: GetShelf :one
SELECT * FROM shelves WHERE id = ?;

-- name: DeleteShelf :exec
DELETE FROM shelves WHERE id = ?;

-- name: RenumberShelf :exec
UPDATE shelves SET position = ? WHERE id = ?;

-- name: ListShelves :many
-- Every shelf in walking order: bookcase by bookcase, each from the top down.
SELECT s.*, b.name AS bookcase, b.position AS bookcase_position
FROM shelves s JOIN bookcases b ON b.id = s.bookcase_id
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
-- Every copy with what places it: its shelving and its size.
SELECT
    cs.*, e.format, e.cover,
    CAST(COALESCE(e.pages, 0) AS INTEGER) AS pages,
    CAST(COALESCE(e.height_mm, 0) AS INTEGER) AS height_mm,
    CAST(COALESCE(e.thickness_mm, 0) AS INTEGER) AS thickness_mm,
    CAST(EXISTS (SELECT 1 FROM loans l WHERE l.copy_id = cs.copy_id AND l.returned_on = '') AS INTEGER) AS on_loan
FROM copy_shelving cs
JOIN editions e ON e.id = cs.edition_id;
