-- name: CreateDocument :one
INSERT INTO documents (kind, title, authors, container, volume, issue, pages, published, publisher, doi, url, course_id, notes, subjects, abstract, editors)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: UpdateDocument :exec
UPDATE documents
SET kind = ?, title = ?, authors = ?, container = ?, volume = ?, issue = ?, pages = ?, published = ?,
    publisher = ?, doi = ?, url = ?, course_id = ?, notes = ?, subjects = ?, abstract = ?,
    editors = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: DeleteDocument :exec
DELETE FROM documents WHERE id = ?;

-- name: GetDocument :one
SELECT d.*, CAST(COALESCE(c.department || ' ' || c.number || ' ' || c.title, '') AS TEXT) AS course_name
FROM documents d LEFT JOIN courses c ON c.id = d.course_id
WHERE d.id = ?;

-- name: DocumentByDOI :one
SELECT id FROM documents WHERE doi = ? AND doi <> '';

-- name: ListDocuments :many
-- Every document, newest first. A household's papers are a few hundred at most, so the list
-- goes whole and is filtered in the browser; the words inside the files are searched here.
SELECT
    d.id, d.kind, d.title, d.authors, d.container, d.published, d.doi, d.course_id, d.created_at,
    CAST(COALESCE(c.department || ' ' || c.number || ' ' || c.title, '') AS TEXT) AS course_name,
    (SELECT count(*) FROM document_files f WHERE f.document_id = d.id) AS file_count,
    (SELECT count(*) FROM document_files f WHERE f.document_id = d.id AND f.text_status = 'pending') AS pending_count
FROM documents d LEFT JOIN courses c ON c.id = d.course_id
ORDER BY d.created_at DESC, d.id DESC;

-- name: ListCourseDocuments :many
SELECT d.id, d.kind, d.title, d.authors, d.published
FROM documents d
WHERE d.course_id = ?
ORDER BY d.published, d.title;

-- name: CreateDocumentFile :one
INSERT INTO document_files (document_id, name, stored, content_type, size, position, text_status)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id;

-- name: NextFilePosition :one
SELECT CAST(COALESCE(max(position), 0) + 1 AS INTEGER) FROM document_files WHERE document_id = ?;

-- name: ListDocumentFiles :many
SELECT * FROM document_files WHERE document_id = ? ORDER BY position, id;

-- name: GetDocumentFile :one
SELECT * FROM document_files WHERE id = ? AND document_id = ?;

-- name: DeleteDocumentFile :exec
DELETE FROM document_files WHERE id = ?;

-- name: SetFileText :exec
UPDATE document_files SET text = ?, text_status = ?, text_error = ? WHERE id = ?;

-- name: NextFileForText :one
-- The oldest file still waiting to be read that has tries left.
SELECT * FROM document_files
WHERE text_status = 'pending' AND text_attempts < 3
ORDER BY id
LIMIT 1;

-- name: CountFileTextTry :exec
UPDATE document_files SET text_attempts = text_attempts + 1 WHERE id = ?;

-- name: DocumentBody :one
-- A document's searchable words: every file's text, in order.
SELECT CAST(COALESCE(group_concat(text, char(10)), '') AS TEXT) AS body
FROM (SELECT text FROM document_files WHERE document_id = ? ORDER BY position, id);

-- name: ResetFileTextTries :execrows
UPDATE document_files SET text_attempts = 0 WHERE id = ?;

-- name: GetDocumentImportRef :one
SELECT document_id FROM document_import_refs WHERE source = ? AND external_id = ?;

-- name: CreateDocumentImportRef :exec
INSERT INTO document_import_refs (source, external_id, document_id) VALUES (?, ?, ?);
