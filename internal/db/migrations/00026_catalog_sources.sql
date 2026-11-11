-- +goose Up

-- HathiTrust and Harvard Library are lookup sources for class numbers and heights the Library
-- of Congress lacks, so the books without a Dewey number, an LC number, or a height
-- go back in the background lookup's queue. It fills blanks only, and the other sources answer
-- from what they said before.
INSERT INTO enrichment (edition_id, status)
SELECT id, 'pending' FROM editions
WHERE isbn13 IS NOT NULL AND (ddc = '' OR lcc = '' OR height_mm IS NULL)
ON CONFLICT (edition_id) DO UPDATE SET
    status = 'pending', attempts = 0, last_error = '',
    next_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now');

-- +goose Down
SELECT 1;
