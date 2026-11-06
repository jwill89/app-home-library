-- +goose Up

-- Each edition's authors in credit order, as every list writes them: one place for the line
-- the library list, the scan list, the loans, and the courses all show.
CREATE VIEW edition_authors AS
SELECT edition_id, CAST(group_concat(name, ', ') AS TEXT) AS authors
FROM (
    SELECT ec.edition_id, p.name
    FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
    WHERE ec.role = 'author'
    ORDER BY ec.edition_id, ec.position
)
GROUP BY edition_id;

DROP VIEW scan_rows;
CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.error, s.scanned_at, s.reviewed,
    CAST(COALESCE(s.edition_id, 0) AS INTEGER) AS edition_id,
    CAST(COALESCE(e.title, '') AS TEXT) AS title,
    CAST(COALESCE(e.cover, '') AS TEXT) AS cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM scans s
LEFT JOIN editions e ON e.id = s.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = s.edition_id;

-- The people the owner lends to, shared by name as series and sections are. A borrower comes
-- into being with their first loan.
CREATE TABLE borrowers (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- One copy lent once. A loan with no return date is a copy still out; returned loans stay as
-- the history.
CREATE TABLE loans (
    id INTEGER PRIMARY KEY,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    borrower_id INTEGER NOT NULL REFERENCES borrowers (id) ON DELETE CASCADE,
    lent_on TEXT NOT NULL,
    due_on TEXT NOT NULL DEFAULT '',
    returned_on TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX loans_copy_idx ON loans (copy_id);
CREATE INDEX loans_borrower_idx ON loans (borrower_id);
-- A copy is with one borrower at a time.
CREATE UNIQUE INDEX loans_out_idx ON loans (copy_id) WHERE returned_on = '';

-- A loan as the lists show it: with its borrower and the book the copy is of.
CREATE VIEW loan_rows AS
SELECT
    l.id, l.copy_id, l.borrower_id, b.name AS borrower, l.lent_on, l.due_on, l.returned_on,
    l.notes, c.edition_id, e.title, e.cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM loans l
JOIN borrowers b ON b.id = l.borrower_id
JOIN copies c ON c.id = l.copy_id
JOIN editions e ON e.id = c.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = c.edition_id;

-- A course a book was used for. The term is a season and a year, so courses sort in the
-- order they were taken.
CREATE TABLE courses (
    id INTEGER PRIMARY KEY,
    school TEXT NOT NULL DEFAULT '',
    level TEXT NOT NULL DEFAULT '' CHECK (level IN ('', 'high_school', 'undergraduate', 'graduate', 'professional', 'other')),
    department TEXT NOT NULL DEFAULT '',
    number TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    term TEXT NOT NULL DEFAULT '' CHECK (term IN ('', 'winter', 'spring', 'summer', 'fall')),
    year INTEGER CHECK (year BETWEEN 1900 AND 2100),
    instructor TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- The copies a course used: the physical book, since that is the one with the notes in it.
CREATE TABLE course_copies (
    course_id INTEGER NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    required INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    PRIMARY KEY (course_id, copy_id)
);

CREATE INDEX course_copies_copy_idx ON course_copies (copy_id);

-- +goose Down
DROP TABLE course_copies;
DROP TABLE courses;
DROP VIEW loan_rows;
DROP TABLE loans;
DROP TABLE borrowers;
DROP VIEW scan_rows;
CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.error, s.scanned_at, s.reviewed,
    CAST(COALESCE(s.edition_id, 0) AS INTEGER) AS edition_id,
    CAST(COALESCE(e.title, '') AS TEXT) AS title,
    CAST(COALESCE(e.cover, '') AS TEXT) AS cover,
    CAST(COALESCE((
        SELECT group_concat(name, ', ') FROM (
            SELECT p.name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
            WHERE ec.edition_id = s.edition_id AND ec.role = 'author' ORDER BY ec.position
        )
    ), '') AS TEXT) AS authors
FROM scans s
LEFT JOIN editions e ON e.id = s.edition_id;
DROP VIEW edition_authors;
