-- name: EnqueueEnrichment :exec
INSERT INTO enrichment (edition_id, fresh) VALUES (?, ?)
ON CONFLICT (edition_id) DO UPDATE
SET status = 'pending', attempts = 0, last_error = '', fresh = excluded.fresh,
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    queued_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- name: EnqueueEveryEnrichment :execrows
-- Every book with an ISBN to look it up by, the sources asked again.
INSERT INTO enrichment (edition_id, fresh)
SELECT id, 1 FROM editions WHERE isbn13 IS NOT NULL
ON CONFLICT (edition_id) DO UPDATE
SET status = 'pending', attempts = 0, last_error = '', fresh = 1,
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    queued_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- name: NextEnrichment :one
-- A book's first lookup goes before one asked again: a book added while the whole library is
-- being looked up again is filled in at once, not after it.
SELECT edition_id, fresh FROM enrichment
WHERE status = 'pending' AND next_at <= ?
ORDER BY fresh, queued_at, edition_id
LIMIT 1;

-- name: FinishEnrichment :exec
UPDATE enrichment SET status = 'done', last_error = '' WHERE edition_id = ?;

-- name: RetryEnrichment :exec
UPDATE enrichment
SET attempts = attempts + 1,
    status = CASE WHEN attempts + 1 >= sqlc.arg(max_attempts) THEN 'failed' ELSE 'pending' END,
    next_at = sqlc.arg(next_at),
    last_error = sqlc.arg(last_error)
WHERE edition_id = sqlc.arg(edition_id);

-- name: GetEnrichment :one
SELECT status, attempts FROM enrichment WHERE edition_id = ?;

-- name: EnrichmentCounts :many
-- A pending lookup not yet due is waiting out its back-off, counted apart from those due now.
SELECT CAST(CASE WHEN status = 'pending' AND next_at > sqlc.arg(now) THEN 'waiting' ELSE status END AS TEXT) AS status,
    count(*) AS n
FROM enrichment
GROUP BY 1;

-- name: GetLookup :one
SELECT body, fetched_at FROM lookups WHERE lookup_key = ? AND source = ?;

-- name: PutLookup :exec
INSERT INTO lookups (lookup_key, source, body) VALUES (?, ?, ?)
ON CONFLICT (lookup_key, source) DO UPDATE
SET body = excluded.body, fetched_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');
