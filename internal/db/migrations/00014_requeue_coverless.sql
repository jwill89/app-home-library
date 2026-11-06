-- +goose Up

-- Hardcover joins the lookup's sources for covers (2026-11-05), so the books without one go
-- back in the background lookup's queue. A cover the owner photographed is never replaced.
INSERT INTO enrichment (edition_id, status)
SELECT id, 'pending' FROM editions
WHERE cover = '' AND isbn13 IS NOT NULL
ON CONFLICT (edition_id) DO UPDATE SET
    status = 'pending', attempts = 0, last_error = '',
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- +goose Down
SELECT 1;
