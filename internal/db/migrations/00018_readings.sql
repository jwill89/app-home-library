-- +goose Up

-- Each earlier read of a work, started and finished. The work keeps the read in progress or
-- the latest; reading it again puts that one here first.
CREATE TABLE readings (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    started_on TEXT NOT NULL DEFAULT '',
    finished_on TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX readings_work_idx ON readings (work_id, finished_on);

-- +goose Down
DROP TABLE readings;
