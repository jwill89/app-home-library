-- +goose Up

-- A book's category: fiction, nonfiction, manga, a light novel, or a graphic novel. Empty
-- is worked out whenever the book is read, from its class number, series, publisher, and
-- subjects, so a better rule sorts every such book again; a category the owner or an import
-- set is kept, "uncategorized" among them.
ALTER TABLE works ADD COLUMN category TEXT NOT NULL DEFAULT ''
    CHECK (category IN ('', 'fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized'));

-- +goose Down
ALTER TABLE works DROP COLUMN category;
