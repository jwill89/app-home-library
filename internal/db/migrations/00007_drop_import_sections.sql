-- +goose Up

-- The owner shelves without sections (2026-11-04). The three the library began with, which the
-- Bookshelf import filled from its shelves, go, and every copy is left without one; its call
-- number then follows the Dewey rule. Sections the owner makes later work as before.
UPDATE copies SET section_id = NULL
WHERE section_id IN (SELECT id FROM sections WHERE name IN ('Fiction', 'Nonfiction', 'Manga and Light Novels'));
DELETE FROM sections WHERE name IN ('Fiction', 'Nonfiction', 'Manga and Light Novels');

-- +goose Down
INSERT OR IGNORE INTO sections (name, shelving, label_prefix) VALUES
    ('Fiction', 'author', 'FIC'), ('Nonfiction', 'dewey', ''), ('Manga and Light Novels', 'series', 'MANGA');
