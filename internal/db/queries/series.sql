-- name: ListSeriesVolumes :many
-- Every series with its volumes, each volume's number, and whether it is only wanted.
SELECT s.id, s.name, w.series_number, e.wanted_on, e.id AS edition_id
FROM series s
JOIN works w ON w.series_id = s.id
JOIN editions e ON e.work_id = w.id
ORDER BY s.name COLLATE NOCASE, s.id;

-- name: GetSeriesByName :one
SELECT * FROM series WHERE name = ?;

-- name: RenameSeries :execrows
UPDATE series SET name = ? WHERE id = ?;

-- name: MoveSeriesWorks :exec
-- One series merged into another: its works go to the other.
UPDATE works SET series_id = sqlc.arg(into), updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE series_id = sqlc.arg(from_id);

-- name: DeleteSeries :exec
DELETE FROM series WHERE id = ?;

-- name: ListSeries :many
-- Every series, named or put aside, for matching a name as the library writes it.
SELECT * FROM series ORDER BY id;

-- name: SetSeriesDismissed :exec
-- A series put aside as no series, on the day given, or taken back with an empty day.
UPDATE series SET dismissed_on = ? WHERE id = ?;

-- name: LeaveSeries :execrows
-- One work out of its series, and so out of its number in it.
UPDATE works SET series_id = NULL, series_number = '', updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = sqlc.arg(work_id) AND series_id = sqlc.arg(series_id);

-- name: EmptySeries :execrows
-- Every work out of a series.
UPDATE works SET series_id = NULL, series_number = '', updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE series_id = ?;

-- name: SetSeriesByOwner :exec
-- A work's editions keep its series as the owner left it, so a lookup does not fill it again.
UPDATE editions SET sources = json_set(sources, '$.series', 'owner', '$.seriesNumber', 'owner')
WHERE work_id = ?;
