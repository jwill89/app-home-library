-- name: ListChanges :many
-- The changes after a cursor, oldest first, a page at a time (events.changesPage).
SELECT id, type, ref FROM changes WHERE id > ? ORDER BY id LIMIT 1000;

-- name: LastChange :one
SELECT CAST(COALESCE(max(id), 0) AS INTEGER) AS id FROM changes;

-- name: ChangeBefore :one
-- The last change made at or before a moment; with none, the one before the oldest kept.
SELECT CAST(COALESCE(
    (SELECT c.id FROM changes AS c WHERE c.at <= sqlc.arg(at) ORDER BY c.id DESC LIMIT 1),
    (SELECT min(c.id) - 1 FROM changes AS c),
    0) AS INTEGER) AS id;

-- name: FirstChange :one
SELECT CAST(COALESCE(min(id), 0) AS INTEGER) AS id FROM changes;

-- name: PruneChanges :execrows
-- The newest changes stay, enough for a page away for a while; a cursor older than them
-- fetches everything again.
DELETE FROM changes WHERE id <= (SELECT max(c.id) FROM changes AS c) - sqlc.arg(keep);
