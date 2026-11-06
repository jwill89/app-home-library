-- +goose Up

-- Where an imported book is in the app it came from (a Goodreads or LibraryThing page), so its
-- page here links back to it. Empty for an app with no web address, as Bookshelf.
ALTER TABLE import_refs ADD COLUMN link TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE import_refs DROP COLUMN link;
