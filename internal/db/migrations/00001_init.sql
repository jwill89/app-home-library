-- +goose Up

-- One owner, signed in by passkey. A passkey belongs to the name it was made on (rp_id),
-- since the site answers on two names and a passkey works on one.
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE passkeys (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    credential_id TEXT NOT NULL UNIQUE,
    credential TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT
);

CREATE INDEX passkeys_user_idx ON passkeys (user_id);

-- A setup code is kept only as its hash and spent on first use.
CREATE TABLE setup_codes (
    code_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL
);

-- scs's sqlite3store reads and writes this table.
CREATE TABLE sessions (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry REAL NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE people (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    sort_name TEXT NOT NULL
);

CREATE TABLE series (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE sections (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE genres (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

-- The book in the abstract. Reading belongs here, so a second copy or edition does not
-- start the count again.
CREATE TABLE works (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    series_id INTEGER REFERENCES series (id) ON DELETE SET NULL,
    series_number TEXT NOT NULL DEFAULT '',
    read_status TEXT NOT NULL DEFAULT 'unread' CHECK (read_status IN ('unread', 'reading', 'read')),
    current_page INTEGER,
    started_on TEXT NOT NULL DEFAULT '',
    finished_on TEXT NOT NULL DEFAULT '',
    rating INTEGER CHECK (rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX works_series_idx ON works (series_id);

CREATE TABLE work_genres (
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (work_id, genre_id)
);

-- One ISBN, or one book without one. published is YYYY, YYYY-MM, or YYYY-MM-DD, as precise
-- as the sources know. sources maps a field to where its value came from, which is how a
-- later lookup knows what it may fill.
CREATE TABLE editions (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    isbn13 TEXT UNIQUE,
    isbn10 TEXT NOT NULL DEFAULT '',
    lccn TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    publish_place TEXT NOT NULL DEFAULT '',
    published TEXT NOT NULL DEFAULT '',
    edition_statement TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '' CHECK (format IN ('', 'hardcover', 'paperback', 'trade_paperback', 'mass_market', 'library_binding', 'spiral', 'board_book', 'leather', 'other')),
    pages INTEGER,
    height_mm INTEGER,
    width_mm INTEGER,
    thickness_mm INTEGER,
    language TEXT NOT NULL DEFAULT '',
    ddc TEXT NOT NULL DEFAULT '',
    lcc TEXT NOT NULL DEFAULT '',
    subjects TEXT NOT NULL DEFAULT '[]',
    description TEXT NOT NULL DEFAULT '',
    cover TEXT NOT NULL DEFAULT '',
    sources TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX editions_work_idx ON editions (work_id);

CREATE TABLE edition_contributors (
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id),
    role TEXT NOT NULL CHECK (role IN ('author', 'editor', 'translator', 'illustrator', 'narrator', 'contributor')),
    position INTEGER NOT NULL,
    PRIMARY KEY (edition_id, role, position)
);

CREATE INDEX edition_contributors_person_idx ON edition_contributors (person_id);

-- The physical book. AUTOINCREMENT because the barcode is made from the id, and a
-- printed barcode must never come back for another copy.
CREATE TABLE copies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    section_id INTEGER REFERENCES sections (id) ON DELETE SET NULL,
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    acquired_on TEXT NOT NULL DEFAULT '',
    acquired_from TEXT NOT NULL DEFAULT '',
    price_cents INTEGER,
    condition TEXT NOT NULL DEFAULT '' CHECK (condition IN ('', 'fine', 'very_good', 'good', 'fair', 'poor')),
    dust_jacket INTEGER CHECK (dust_jacket IN (0, 1)),
    signed INTEGER NOT NULL DEFAULT 0 CHECK (signed IN (0, 1)),
    location TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX copies_edition_idx ON copies (edition_id);

-- The record a copy came from in another app, so the same file imported twice adds nothing.
CREATE TABLE import_refs (
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    PRIMARY KEY (source, external_id)
);

-- Editions waiting for a lookup to fill their blanks, worked through in the background.
CREATE TABLE enrichment (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_error TEXT NOT NULL DEFAULT '',
    queued_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX enrichment_due_idx ON enrichment (status, next_at);

-- Each source's raw answer, so the merge can be run again without asking the source again.
CREATE TABLE lookups (
    lookup_key TEXT NOT NULL,
    source TEXT NOT NULL,
    fetched_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    body BLOB NOT NULL,
    PRIMARY KEY (lookup_key, source)
);

INSERT INTO sections (name) VALUES ('Fiction'), ('Nonfiction'), ('Manga and Light Novels');

-- +goose Down
DROP TABLE lookups;
DROP TABLE enrichment;
DROP TABLE import_refs;
DROP TABLE copies;
DROP TABLE edition_contributors;
DROP TABLE editions;
DROP TABLE work_genres;
DROP TABLE works;
DROP TABLE genres;
DROP TABLE sections;
DROP TABLE series;
DROP TABLE people;
DROP TABLE sessions;
DROP TABLE setup_codes;
DROP TABLE passkeys;
DROP TABLE users;
