-- +goose Up

-- Corner braces: blocks under the shelf above, at both ends, the whole depth of the shelf. A
-- book standing under one has the brace's height less room, so the ends of a braced shelf
-- take only the shorter books. Millimetres; 0 is no brace.
ALTER TABLE shelves ADD COLUMN brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200);
ALTER TABLE shelves ADD COLUMN brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200);

-- +goose Down
ALTER TABLE shelves DROP COLUMN brace_width_mm;
ALTER TABLE shelves DROP COLUMN brace_height_mm;
