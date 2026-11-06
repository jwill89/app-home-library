-- +goose Up

-- Marking a book read gave it the day it was marked as the day it was finished, and the owner
-- marked their past reads so (2026-11-04 and 05), putting them all in 2026. Marking a book read
-- now gives it no day; the reads so marked, with no start, go back to undated. A read the owner
-- dated by hand those days is the rare one, and the nightly backup keeps the days as they were.
UPDATE works SET finished_on = ''
WHERE read_status = 'read' AND started_on = '' AND finished_on IN ('2026-11-04', '2026-11-05');

-- +goose Down
SELECT 1;
