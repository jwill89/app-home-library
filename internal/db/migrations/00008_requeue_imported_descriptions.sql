-- +goose Up

-- The Bookshelf export wrote markup as &&L and &&R, and cut descriptions off at 1500
-- characters. The background lookup now cleans the one and replaces the other with a source's
-- whole description, so the books that have either go back in its queue (2026-11-04).
INSERT INTO enrichment (edition_id, status)
SELECT id, 'pending' FROM editions
WHERE description LIKE '%&&L%'
   OR (length(description) = 1500 AND json_extract(sources, '$.description') = 'bookshelf')
ON CONFLICT (edition_id) DO UPDATE SET
    status = 'pending', attempts = 0, last_error = '',
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- +goose Down
SELECT 1;
