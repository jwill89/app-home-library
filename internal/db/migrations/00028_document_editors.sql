-- +goose Up

-- A chapter's book has editors, which its citation names ("In Book, edited by …"): a JSON
-- list of names, as the authors are (2026-11-05). The search finds them with the authors.
ALTER TABLE documents ADD COLUMN editors TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE documents DROP COLUMN editors;
