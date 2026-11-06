-- +goose Up

-- What each share link shows (2026-11-05). Its books: every one, some (those of the categories
-- chosen, and the books chosen one by one), or none, with the wanted ones among them or not.
-- Its papers: every one, some (a course's, and the papers chosen), or none. A paper's own
-- "shared" switch, which every link followed alike, becomes each link's choice of papers.
ALTER TABLE shares ADD COLUMN books TEXT NOT NULL DEFAULT 'all' CHECK (books IN ('all', 'some', 'none'));
ALTER TABLE shares ADD COLUMN wishlist INTEGER NOT NULL DEFAULT 1 CHECK (wishlist IN (0, 1));
ALTER TABLE shares ADD COLUMN papers TEXT NOT NULL DEFAULT 'none' CHECK (papers IN ('all', 'some', 'none'));

CREATE TABLE share_categories (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (share_id, category)
) WITHOUT ROWID;

-- A book or paper removed leaves every link; one added later under its number is in none.
CREATE TABLE share_books (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, edition_id)
) WITHOUT ROWID;
CREATE INDEX share_books_edition_idx ON share_books (edition_id);

CREATE TABLE share_courses (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    course_id INTEGER NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, course_id)
) WITHOUT ROWID;
CREATE INDEX share_courses_course_idx ON share_courses (course_id);

CREATE TABLE share_documents (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, document_id)
) WITHOUT ROWID;
CREATE INDEX share_documents_document_idx ON share_documents (document_id);

-- Every link showed the papers marked shared: each keeps them as its own choice.
INSERT INTO share_documents (share_id, document_id)
SELECT s.id, d.id FROM shares s, documents d WHERE d.shared = 1;
UPDATE shares SET papers = 'some' WHERE id IN (SELECT share_id FROM share_documents);

ALTER TABLE documents DROP COLUMN shared;

-- +goose Down
ALTER TABLE documents ADD COLUMN shared INTEGER NOT NULL DEFAULT 0 CHECK (shared IN (0, 1));
UPDATE documents SET shared = 1 WHERE id IN (SELECT document_id FROM share_documents);
DROP TABLE share_documents;
DROP TABLE share_courses;
DROP TABLE share_books;
DROP TABLE share_categories;
ALTER TABLE shares DROP COLUMN papers;
ALTER TABLE shares DROP COLUMN wishlist;
ALTER TABLE shares DROP COLUMN books;
