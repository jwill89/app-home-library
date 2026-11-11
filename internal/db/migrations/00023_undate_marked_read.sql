-- +goose Up

-- Reads marked read, with no start, go back to undated. Marking a book read gives it no day,
-- since the day it was marked is not the day it was finished: the past reads the owner marked
-- in a batch had all landed in 2026. A read the owner dated by hand in those days is the
-- rare one, and the nightly backup keeps the days as they were.
UPDATE works SET finished_on = ''
WHERE read_status = 'read' AND started_on = '' AND finished_on IN ('2026-11-04', '2026-11-05');

-- +goose Down
SELECT 1;
