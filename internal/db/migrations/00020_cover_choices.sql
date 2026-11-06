-- +goose Up

-- A lookup's covers now say which source each came from (2026-11-05), for the cover chooser to
-- name. The lookups kept in the scan queue take the new shape; Open Library and Hardcover are
-- the only sources a lookup offers covers from.
UPDATE scans SET proposal = json_set(proposal, '$.covers', json((
    SELECT json_group_array(json_object(
        'url', c.value,
        'source', CASE WHEN c.value LIKE '%hardcover.app%' THEN 'hardcover' ELSE 'openlibrary' END
    ))
    FROM json_each(scans.proposal, '$.covers') AS c
)))
WHERE json_valid(proposal) AND json_type(proposal, '$.covers') = 'array';

-- +goose Down
SELECT 1;
