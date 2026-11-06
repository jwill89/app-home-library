-- name: CreateShare :one
INSERT INTO shares (name, token_hash, expires_at, books, wishlist, papers) VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateShare :one
UPDATE shares SET name = ?, expires_at = ?, books = ?, wishlist = ?, papers = ? WHERE id = ?
RETURNING *;

-- name: ListShares :many
SELECT * FROM shares ORDER BY id DESC;

-- name: ListShareScope :many
-- A link's choices, a row each: a category by its name, or a book, course, or paper by number.
SELECT CAST('category' AS TEXT) AS kind, CAST(0 AS INTEGER) AS ref, category AS name
FROM share_categories WHERE share_categories.share_id = sqlc.arg(share_id)
UNION ALL
SELECT 'book', edition_id, '' FROM share_books WHERE share_books.share_id = sqlc.arg(share_id)
UNION ALL
SELECT 'course', course_id, '' FROM share_courses WHERE share_courses.share_id = sqlc.arg(share_id)
UNION ALL
SELECT 'paper', document_id, '' FROM share_documents WHERE share_documents.share_id = sqlc.arg(share_id);

-- name: ClearShareCategories :exec
DELETE FROM share_categories WHERE share_id = ?;

-- name: ClearShareBooks :exec
DELETE FROM share_books WHERE share_id = ?;

-- name: ClearShareCourses :exec
DELETE FROM share_courses WHERE share_id = ?;

-- name: ClearShareDocuments :exec
DELETE FROM share_documents WHERE share_id = ?;

-- name: AddShareCategory :exec
INSERT INTO share_categories (share_id, category) VALUES (?, ?);

-- The three below take a number only while it names a record: one removed while the owner
-- was choosing is passed over.

-- name: AddShareBook :exec
INSERT INTO share_books (share_id, edition_id)
SELECT sqlc.arg(share_id), id FROM editions WHERE id = sqlc.arg(edition_id);

-- name: AddShareCourse :exec
INSERT INTO share_courses (share_id, course_id)
SELECT sqlc.arg(share_id), id FROM courses WHERE id = sqlc.arg(course_id);

-- name: AddShareDocument :exec
INSERT INTO share_documents (share_id, document_id)
SELECT sqlc.arg(share_id), id FROM documents WHERE id = sqlc.arg(document_id);

-- name: DeleteShare :execrows
DELETE FROM shares WHERE id = ?;

-- name: ShareByToken :one
-- A link still running: not removed, and not past its end, if it has one.
SELECT * FROM shares
WHERE token_hash = sqlc.arg(token_hash) AND (expires_at = '' OR expires_at > sqlc.arg(now));


-- name: TouchShare :exec
UPDATE shares SET last_used_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?;

-- name: DeleteCalendarLinks :exec
DELETE FROM calendar_links;

-- name: CreateCalendarLink :one
INSERT INTO calendar_links (token_hash) VALUES (?) RETURNING *;

-- name: GetCalendarLink :one
SELECT * FROM calendar_links ORDER BY id DESC LIMIT 1;

-- name: CalendarLinkByToken :one
SELECT * FROM calendar_links WHERE token_hash = ?;

-- name: TouchCalendarLink :exec
UPDATE calendar_links SET last_used_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?;
