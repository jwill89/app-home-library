-- name: QueueLabel :exec
INSERT OR IGNORE INTO label_queue (copy_id, kind) VALUES (?, ?);

-- name: UnqueueLabel :execrows
DELETE FROM label_queue WHERE copy_id = ? AND kind = ?;

-- name: UnqueueCopyLabels :exec
DELETE FROM label_queue WHERE copy_id = ?;

-- name: ListLabelQueue :many
SELECT q.kind, q.queued_at, cs.*
FROM label_queue q
JOIN copy_shelving cs ON cs.copy_id = q.copy_id
ORDER BY q.copy_id;

-- name: CountLabelQueue :many
-- How many labels of each kind wait: the rows ListLabelQueue lists, counted.
SELECT q.kind, count(*) AS n
FROM label_queue q
JOIN copy_shelving cs ON cs.copy_id = q.copy_id
GROUP BY q.kind;

-- name: ListEditionLabels :many
-- The labels a book's copies wait on, of the kinds that are printed.
SELECT q.copy_id, q.kind
FROM label_queue q
JOIN copies c ON c.id = q.copy_id
JOIN label_offsets lo ON lo.kind = q.kind AND lo.enabled = 1
WHERE c.edition_id = ?
ORDER BY q.copy_id, q.kind;

-- name: ListEditionShelving :many
SELECT * FROM copy_shelving WHERE edition_id = ? ORDER BY copy_id;

-- name: ListLabelOffsets :many
SELECT * FROM label_offsets ORDER BY kind;

-- name: SetLabelSheet :execrows
UPDATE label_offsets SET x_mm = ?, y_mm = ?, enabled = ? WHERE kind = ?;

-- name: ListShelvingRules :many
SELECT s.*, (SELECT count(*) FROM copies c WHERE c.section_id = s.id) AS copies
FROM sections s
ORDER BY s.name;

-- name: GetShelvingRule :one
SELECT s.*, (SELECT count(*) FROM copies c WHERE c.section_id = s.id) AS copies
FROM sections s
WHERE s.id = ?;

-- name: SetShelvingRule :execrows
UPDATE sections SET shelving = ?, label_prefix = ? WHERE id = ?;
