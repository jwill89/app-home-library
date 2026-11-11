-- +goose Up

-- Google's lookup reads a volume's whole record, which has the height, width, and thickness
-- its search answer leaves out. Its kept answers are the search's, so they go, and the books
-- lacking a measurement go back in the background lookup's queue, which fills blanks only: a
-- measurement the owner typed or a source gave stands.
DELETE FROM lookups WHERE source = 'google';

INSERT INTO enrichment (edition_id, status)
SELECT id, 'pending' FROM editions
WHERE isbn13 IS NOT NULL AND (height_mm IS NULL OR width_mm IS NULL OR thickness_mm IS NULL)
ON CONFLICT (edition_id) DO UPDATE SET
    status = 'pending', attempts = 0, last_error = '',
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- +goose Down
SELECT 1;
