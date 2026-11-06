-- name: CreateScan :one
INSERT INTO scans (isbn13, status, edition_id, done_at)
VALUES (?, ?, ?, ?)
RETURNING id;

-- name: OpenScanForISBN :one
-- A scan of the same book still on the list and not yet settled: a second scan of it is the
-- same one.
SELECT id FROM scans
WHERE isbn13 = ? AND reviewed = 0 AND status IN ('pending', 'found', 'adding', 'added')
ORDER BY id DESC
LIMIT 1;

-- name: GetScan :one
SELECT * FROM scans WHERE id = ?;

-- name: ListOpenScans :many
SELECT * FROM scan_rows WHERE reviewed = 0 ORDER BY id DESC LIMIT 300;

-- name: GetScanView :one
SELECT * FROM scan_rows WHERE id = ?;

-- name: NextScanWork :one
-- The oldest scan the background owes something: a lookup, or the adding the owner asked for.
SELECT * FROM scans WHERE status IN ('pending', 'adding') ORDER BY id LIMIT 1;

-- name: FindScan :exec
UPDATE scans SET status = 'found', proposal = ?, attempts = 0, error = '' WHERE id = ?;

-- name: FinishScan :exec
UPDATE scans
SET status = ?, edition_id = ?, error = '', done_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;

-- name: AddFoundScan :execrows
UPDATE scans SET status = 'adding', section = ?, acquired_as = ?, want = ?
WHERE id = ? AND status = 'found' AND reviewed = 0;

-- name: RetryScan :exec
-- A lookup or an adding that could not reach what it needed is tried again, then left failed.
UPDATE scans
SET attempts = attempts + 1,
    status = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts) THEN 'failed' ELSE status END,
    error = sqlc.arg(error)
WHERE id = sqlc.arg(id);

-- name: ReviewScan :execrows
UPDATE scans SET reviewed = 1 WHERE id = ? AND status NOT IN ('pending', 'adding');

-- name: ReviewFinishedScans :exec
UPDATE scans SET reviewed = 1 WHERE reviewed = 0 AND status NOT IN ('pending', 'adding');

-- name: DeleteScan :exec
DELETE FROM scans WHERE id = ?;

-- name: CountPendingScans :one
SELECT count(*) FROM scans WHERE status IN ('pending', 'adding');
