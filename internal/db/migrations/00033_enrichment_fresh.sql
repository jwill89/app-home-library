-- +goose Up

-- A lookup the owner asked for asks each source again; one queued for a new book or an import
-- is answered from what the sources said last time, where they said anything.
ALTER TABLE enrichment ADD COLUMN fresh INTEGER NOT NULL DEFAULT 0 CHECK (fresh IN (0, 1));

-- +goose Down
ALTER TABLE enrichment DROP COLUMN fresh;
