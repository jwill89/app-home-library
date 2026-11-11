-- +goose Up

-- Each kind of label is printed or not. Barcode labels go inside the front cover; spine labels
-- wait until the owner settles on a shelving scheme, so they start off, and their
-- queue keeps filling meanwhile, ready when they are turned on.
ALTER TABLE label_offsets ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1));
UPDATE label_offsets SET enabled = 0 WHERE kind = 'spine';

-- +goose Down
ALTER TABLE label_offsets DROP COLUMN enabled;
