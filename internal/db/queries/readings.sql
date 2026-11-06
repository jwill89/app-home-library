-- name: CreateReading :exec
INSERT INTO readings (work_id, started_on, finished_on) VALUES (?, ?, ?);

-- name: ListWorkReadings :many
SELECT * FROM readings WHERE work_id = ? ORDER BY finished_on DESC, id DESC;

-- name: DeleteReading :execrows
DELETE FROM readings WHERE id = ?;

-- name: ReadAgain :exec
-- A work begun again: what it was is kept as a reading first, by the one calling.
UPDATE works
SET read_status = 'reading', current_page = NULL, started_on = ?, finished_on = '',
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: ListFinishedReads :many
-- Every finished read, the latest of each work and the earlier ones alike, with the book it
-- was read in: the work's first edition.
SELECT
    w.id AS work_id, w.finished_on,
    CAST((SELECT min(e.id) FROM editions e WHERE e.work_id = w.id) AS INTEGER) AS book_id,
    CAST(COALESCE((SELECT e.title FROM editions e WHERE e.work_id = w.id ORDER BY e.id LIMIT 1), w.title) AS TEXT) AS title,
    CAST(COALESCE((SELECT e.pages FROM editions e WHERE e.work_id = w.id ORDER BY e.id LIMIT 1), 0) AS INTEGER) AS pages
FROM works w
WHERE w.read_status = 'read' AND w.finished_on <> ''
UNION ALL
SELECT
    r.work_id, r.finished_on,
    CAST((SELECT min(e.id) FROM editions e WHERE e.work_id = r.work_id) AS INTEGER) AS book_id,
    CAST(COALESCE((SELECT e.title FROM editions e WHERE e.work_id = r.work_id ORDER BY e.id LIMIT 1), '') AS TEXT) AS title,
    CAST(COALESCE((SELECT e.pages FROM editions e WHERE e.work_id = r.work_id ORDER BY e.id LIMIT 1), 0) AS INTEGER) AS pages
FROM readings r
WHERE r.finished_on <> ''
ORDER BY finished_on DESC;

-- name: SetReadStatus :exec
-- A work's reading status alone; the days it began and ended are the caller's to keep or set.
UPDATE works
SET read_status = ?, started_on = ?, finished_on = ?,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: SetSeriesReadStatus :execrows
-- Every work of a series the house has (not only wanted), marked read or unread; the days
-- each began and ended are left as they are.
UPDATE works
SET read_status = sqlc.arg(status), updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE series_id = sqlc.arg(series_id) AND read_status <> sqlc.arg(other_than)
    AND EXISTS (SELECT 1 FROM editions e WHERE e.work_id = works.id AND e.wanted_on = '');
