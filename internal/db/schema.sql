-- The schema the migrations build, written by TestSchema (task gen) and never by hand.

CREATE TABLE area_authors (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
) WITHOUT ROWID;

CREATE TABLE "area_categories" (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (area_id, category)
) WITHOUT ROWID;

CREATE TABLE area_order (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    rank INTEGER NOT NULL CHECK (rank >= 1),
    key TEXT NOT NULL CHECK (key IN ('subject', 'author', 'series', 'title', 'published', 'height')),
    PRIMARY KEY (area_id, rank)
) WITHOUT ROWID;

CREATE TABLE area_series (
    area_id INTEGER NOT NULL REFERENCES areas (id) ON DELETE CASCADE,
    series_id INTEGER NOT NULL REFERENCES series (id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, series_id)
) WITHOUT ROWID;

CREATE TABLE "areas" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    position INTEGER NOT NULL,
    dewey_from TEXT NOT NULL DEFAULT '',
    dewey_to TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    min_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (min_height_mm BETWEEN 0 AND 1000),
    max_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (max_height_mm BETWEEN 0 AND 1000),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE "bookcases" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE "borrowers" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    contact TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE "calendar_links" (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME
        CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at))
);

CREATE TABLE "changes" (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('book', 'scan', 'document', 'lookup', 'loan', 'label')),
    ref INTEGER NOT NULL,
    at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (at IS strftime('%Y-%m-%dT%H:%M:%SZ', at))
);

CREATE TABLE "copies" (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    section_id INTEGER REFERENCES sections (id) ON DELETE SET NULL,
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    acquired_on DATE
        CHECK (acquired_on IS date(acquired_on)),
    acquired_from TEXT NOT NULL DEFAULT '',
    price_cents INTEGER,
    condition TEXT NOT NULL DEFAULT '' CHECK (condition IN ('', 'fine', 'very_good', 'good', 'fair', 'poor')),
    dust_jacket INTEGER CHECK (dust_jacket IN (0, 1)),
    signed INTEGER NOT NULL DEFAULT 0 CHECK (signed IN (0, 1)),
    location TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    gone_as TEXT NOT NULL DEFAULT '' CHECK (gone_as IN ('', 'lost', 'sold', 'donated', 'given', 'discarded')),
    gone_on DATE
        CHECK (gone_on IS date(gone_on)),
    gone_note TEXT NOT NULL DEFAULT '',
    seen_on DATE
        CHECK (seen_on IS date(seen_on))
);

CREATE TABLE course_copies (
    course_id INTEGER NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    required INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    PRIMARY KEY (course_id, copy_id)
);

CREATE TABLE "courses" (
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
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE declined_suggestions (
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    field TEXT NOT NULL CHECK (field IN (
        'title', 'subtitle', 'contributors', 'series', 'seriesNumber', 'lccn', 'publisher',
        'publishPlace', 'published', 'editionStatement', 'format', 'pages', 'heightMm', 'widthMm',
        'thicknessMm', 'language', 'ddc', 'lcc', 'description'
    )),
    value TEXT NOT NULL,
    PRIMARY KEY (edition_id, field)
) WITHOUT ROWID;

CREATE TABLE "document_files" (
    id INTEGER PRIMARY KEY,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    stored TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png', 'image/webp')),
    size INTEGER NOT NULL,
    position INTEGER NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    text_status TEXT NOT NULL DEFAULT 'pending' CHECK (text_status IN ('pending', 'done', 'empty', 'failed', 'owner')),
    text_attempts INTEGER NOT NULL DEFAULT 0,
    text_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);

CREATE TABLE document_import_refs (
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    PRIMARY KEY (source, external_id)
);

CREATE VIRTUAL TABLE document_search USING fts5 (
    title, authors, container, subjects, abstract, notes, body,
    tokenize = 'porter unicode61 remove_diacritics 2'
);

CREATE TABLE "documents" (
    id INTEGER PRIMARY KEY,
    kind TEXT NOT NULL DEFAULT 'other' CHECK (kind IN ('article', 'chapter', 'review', 'handout', 'syllabus', 'notes', 'exam', 'other')),
    title TEXT NOT NULL,
    authors TEXT NOT NULL DEFAULT '[]',
    container TEXT NOT NULL DEFAULT '',
    volume TEXT NOT NULL DEFAULT '',
    issue TEXT NOT NULL DEFAULT '',
    pages TEXT NOT NULL DEFAULT '',
    published TEXT NOT NULL DEFAULT '',
    publisher TEXT NOT NULL DEFAULT '',
    doi TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL DEFAULT '',
    course_id INTEGER REFERENCES courses (id) ON DELETE SET NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    subjects TEXT NOT NULL DEFAULT '[]',
    abstract TEXT NOT NULL DEFAULT '',
    editors TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE edition_contributors (
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    person_id INTEGER NOT NULL REFERENCES people (id),
    role TEXT NOT NULL CHECK (role IN ('author', 'editor', 'translator', 'illustrator', 'narrator', 'contributor')),
    position INTEGER NOT NULL,
    PRIMARY KEY (edition_id, role, position)
);

CREATE TABLE "editions" (
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
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    wanted_on DATE
        CHECK (wanted_on IS date(wanted_on))
);

CREATE TABLE "enrichment" (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (next_at IS strftime('%Y-%m-%dT%H:%M:%SZ', next_at)),
    last_error TEXT NOT NULL DEFAULT '',
    queued_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (queued_at IS strftime('%Y-%m-%dT%H:%M:%SZ', queued_at)),
    fresh INTEGER NOT NULL DEFAULT 0 CHECK (fresh IN (0, 1))
);

CREATE TABLE genres (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE import_refs (
    source TEXT NOT NULL,
    external_id TEXT NOT NULL,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE, link TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (source, external_id)
);

CREATE TABLE label_offsets (
    kind TEXT PRIMARY KEY CHECK (kind IN ('barcode', 'spine')),
    x_mm INTEGER NOT NULL DEFAULT 0 CHECK (x_mm BETWEEN -10 AND 10),
    y_mm INTEGER NOT NULL DEFAULT 0 CHECK (y_mm BETWEEN -10 AND 10)
, enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)));

CREATE TABLE "label_queue" (
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('barcode', 'spine')),
    queued_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (queued_at IS strftime('%Y-%m-%dT%H:%M:%SZ', queued_at)),
    PRIMARY KEY (copy_id, kind)
);

CREATE TABLE "layout_settings" (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    fill_percent INTEGER NOT NULL DEFAULT 80 CHECK (fill_percent BETWEEN 50 AND 100),
    applied_at DATETIME
        CHECK (applied_at IS strftime('%Y-%m-%dT%H:%M:%SZ', applied_at))
);

CREATE TABLE "loans" (
    id INTEGER PRIMARY KEY,
    copy_id INTEGER NOT NULL REFERENCES copies (id) ON DELETE CASCADE,
    borrower_id INTEGER NOT NULL REFERENCES borrowers (id) ON DELETE CASCADE,
    lent_on DATE NOT NULL CHECK (lent_on IS date(lent_on)),
    due_on DATE CHECK (due_on IS date(due_on)),
    returned_on DATE CHECK (returned_on IS date(returned_on)),
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE lookup_proposals (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    proposal TEXT NOT NULL
);

CREATE TABLE "lookups" (
    lookup_key TEXT NOT NULL,
    source TEXT NOT NULL,
    fetched_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (fetched_at IS strftime('%Y-%m-%dT%H:%M:%SZ', fetched_at)),
    body BLOB NOT NULL,
    PRIMARY KEY (lookup_key, source)
);

CREATE TABLE "passkeys" (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    rp_id TEXT NOT NULL,
    credential_id TEXT NOT NULL UNIQUE,
    credential TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME
        CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at))
);

CREATE TABLE people (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    sort_name TEXT NOT NULL
);

CREATE TABLE "readings" (
    id INTEGER PRIMARY KEY,
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    started_on DATE
        CHECK (started_on IS date(started_on)),
    finished_on DATE
        CHECK (finished_on IS date(finished_on)),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);

CREATE TABLE "scans" (
    id INTEGER PRIMARY KEY,
    isbn13 TEXT NOT NULL,
    section TEXT NOT NULL DEFAULT '',
    acquired_as TEXT NOT NULL DEFAULT '' CHECK (acquired_as IN ('', 'new', 'used')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'found', 'adding', 'added', 'owned', 'not_found', 'failed')),
    proposal TEXT NOT NULL DEFAULT '',
    edition_id INTEGER REFERENCES editions (id) ON DELETE SET NULL,
    error TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    reviewed INTEGER NOT NULL DEFAULT 0 CHECK (reviewed IN (0, 1)),
    scanned_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (scanned_at IS strftime('%Y-%m-%dT%H:%M:%SZ', scanned_at)),
    done_at DATETIME
        CHECK (done_at IS strftime('%Y-%m-%dT%H:%M:%SZ', done_at)),
    want INTEGER NOT NULL DEFAULT 0 CHECK (want IN (0, 1))
);

CREATE TABLE sections (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
, shelving TEXT NOT NULL DEFAULT 'dewey' CHECK (shelving IN ('dewey', 'author', 'series')), label_prefix TEXT NOT NULL DEFAULT '');

CREATE TABLE "series" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    dismissed_on DATE
        CHECK (dismissed_on IS date(dismissed_on))
);

CREATE TABLE "sessions" (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry DATETIME NOT NULL
        CHECK (expiry IS strftime('%Y-%m-%dT%H:%M:%SZ', expiry))
);

CREATE TABLE "setup_codes" (
    code_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL
        CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at))
);

CREATE TABLE share_books (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    edition_id INTEGER NOT NULL REFERENCES editions (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, edition_id)
) WITHOUT ROWID;

CREATE TABLE "share_categories" (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    category TEXT NOT NULL
        CHECK (category IN ('fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized')),
    PRIMARY KEY (share_id, category)
) WITHOUT ROWID;

CREATE TABLE share_courses (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    course_id INTEGER NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, course_id)
) WITHOUT ROWID;

CREATE TABLE share_documents (
    share_id INTEGER NOT NULL REFERENCES shares (id) ON DELETE CASCADE,
    document_id INTEGER NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
    PRIMARY KEY (share_id, document_id)
) WITHOUT ROWID;

CREATE TABLE "shares" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at DATETIME CHECK (expires_at IS strftime('%Y-%m-%dT%H:%M:%SZ', expires_at)),
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    last_used_at DATETIME CHECK (last_used_at IS strftime('%Y-%m-%dT%H:%M:%SZ', last_used_at)),
    books TEXT NOT NULL DEFAULT 'all' CHECK (books IN ('all', 'some', 'none')),
    wishlist INTEGER NOT NULL DEFAULT 1 CHECK (wishlist IN (0, 1)),
    papers TEXT NOT NULL DEFAULT 'none' CHECK (papers IN ('all', 'some', 'none')),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE "shelves" (
    id INTEGER PRIMARY KEY,
    bookcase_id INTEGER NOT NULL REFERENCES bookcases (id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    width_mm INTEGER NOT NULL CHECK (width_mm BETWEEN 100 AND 5000),
    height_mm INTEGER NOT NULL CHECK (height_mm BETWEEN 50 AND 1000),
    depth_mm INTEGER NOT NULL CHECK (depth_mm BETWEEN 50 AND 1000),
    starts_at TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')) CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    brace_height_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_height_mm BETWEEN 0 AND 200),
    brace_width_mm INTEGER NOT NULL DEFAULT 0 CHECK (brace_width_mm BETWEEN 0 AND 200),
    area_id INTEGER REFERENCES areas (id) ON DELETE SET NULL,
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at))
);

CREATE TABLE "users" (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
);

CREATE TABLE work_genres (
    work_id INTEGER NOT NULL REFERENCES works (id) ON DELETE CASCADE,
    genre_id INTEGER NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (work_id, genre_id)
);

CREATE TABLE "works" (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    series_id INTEGER REFERENCES series (id) ON DELETE SET NULL,
    series_number TEXT NOT NULL DEFAULT '',
    read_status TEXT NOT NULL DEFAULT 'unread' CHECK (read_status IN ('unread', 'reading', 'read')),
    current_page INTEGER,
    started_on DATE
        CHECK (started_on IS date(started_on)),
    finished_on DATE
        CHECK (finished_on IS date(finished_on)),
    rating INTEGER CHECK (rating BETWEEN 1 AND 5),
    notes TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at)),
    updated_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (updated_at IS strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)),
    category TEXT NOT NULL DEFAULT ''
        CHECK (category IN ('', 'fiction', 'nonfiction', 'textbook', 'manga', 'light_novel', 'graphic_novel', 'uncategorized'))
);

CREATE VIEW copy_shelving AS
SELECT
    c.id AS copy_id,
    c.edition_id,
    CAST(COALESCE(sec.name, '') AS TEXT) AS section,
    CAST(COALESCE(sec.shelving, 'dewey') AS TEXT) AS shelving,
    CAST(COALESCE(sec.label_prefix, '') AS TEXT) AS label_prefix,
    e.ddc,
    e.title,
    CAST(COALESCE(s.name, '') AS TEXT) AS series,
    w.series_number,
    CAST(COALESCE((
        SELECT p.sort_name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
        WHERE ec.edition_id = c.edition_id AND ec.role = 'author'
        ORDER BY ec.position
        LIMIT 1
    ), '') AS TEXT) AS author_sort
FROM copies c
JOIN editions e ON e.id = c.edition_id
JOIN works w ON w.id = e.work_id
LEFT JOIN series s ON s.id = w.series_id
LEFT JOIN sections sec ON sec.id = c.section_id
WHERE c.gone_as = '';

CREATE VIEW edition_authors AS
SELECT
    e.id AS edition_id,
    CAST(COALESCE((
        SELECT group_concat(name, ', ')
        FROM (
            SELECT p.name
            FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
            WHERE ec.edition_id = e.id AND ec.role = 'author'
            ORDER BY ec.position
        )
    ), '') AS TEXT) AS authors
FROM editions e;

CREATE VIEW loan_rows AS
SELECT
    l.id, l.copy_id, l.borrower_id, b.name AS borrower, l.lent_on, l.due_on, l.returned_on,
    l.notes, l.updated_at, c.edition_id, e.title, e.cover,
    CAST(COALESCE(ea.authors, '') AS TEXT) AS authors
FROM loans l
JOIN borrowers b ON b.id = l.borrower_id
JOIN copies c ON c.id = l.copy_id
JOIN editions e ON e.id = c.edition_id
LEFT JOIN edition_authors ea ON ea.edition_id = c.edition_id;

CREATE VIEW scan_rows AS
SELECT
    s.id, s.isbn13, s.section, s.status, s.proposal, s.error, s.scanned_at, s.reviewed,
    CAST(COALESCE(s.edition_id, 0) AS INTEGER) AS edition_id,
    CAST(COALESCE(e.title, '') AS TEXT) AS title,
    CAST(COALESCE(e.cover, '') AS TEXT) AS cover,
    CAST(COALESCE(e.wanted_on, '') AS TEXT) AS wanted_on,
    CAST(COALESCE((
        SELECT group_concat(name, ', ') FROM (
            SELECT p.name FROM edition_contributors ec JOIN people p ON p.id = ec.person_id
            WHERE ec.edition_id = s.edition_id AND ec.role = 'author' ORDER BY ec.position
        )
    ), '') AS TEXT) AS authors
FROM scans s
LEFT JOIN editions e ON e.id = s.edition_id;

CREATE INDEX area_authors_person_idx ON area_authors (person_id);

CREATE INDEX area_series_series_idx ON area_series (series_id);

CREATE INDEX copies_edition_idx ON copies (edition_id);

CREATE INDEX course_copies_copy_idx ON course_copies (copy_id);

CREATE INDEX document_files_document_idx ON document_files (document_id, position);

CREATE INDEX document_files_text_idx ON document_files (text_status, id);

CREATE INDEX documents_course_idx ON documents (course_id);

CREATE UNIQUE INDEX documents_doi_idx ON documents (doi) WHERE doi <> '';

CREATE INDEX edition_contributors_person_idx ON edition_contributors (person_id);

CREATE INDEX editions_work_idx ON editions (work_id);

CREATE INDEX enrichment_due_idx ON enrichment (status, next_at);

CREATE INDEX loans_borrower_idx ON loans (borrower_id);

CREATE INDEX loans_copy_idx ON loans (copy_id);

CREATE UNIQUE INDEX loans_out_idx ON loans (copy_id) WHERE returned_on IS NULL;

CREATE INDEX passkeys_user_idx ON passkeys (user_id);

CREATE INDEX readings_work_idx ON readings (work_id, finished_on);

CREATE INDEX scans_open_idx ON scans (reviewed, id);

CREATE INDEX scans_status_idx ON scans (status, id);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE INDEX share_books_edition_idx ON share_books (edition_id);

CREATE INDEX share_courses_course_idx ON share_courses (course_id);

CREATE INDEX share_documents_document_idx ON share_documents (document_id);

CREATE INDEX shelves_area_idx ON shelves (area_id);

CREATE INDEX shelves_bookcase_idx ON shelves (bookcase_id, position);

CREATE INDEX works_series_idx ON works (series_id);

CREATE TRIGGER copies_changed_delete AFTER DELETE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.edition_id);
END;

CREATE TRIGGER copies_changed_insert AFTER INSERT ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER copies_changed_update AFTER UPDATE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER course_copies_changed_delete AFTER DELETE ON course_copies BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e
    WHERE e.work_id = (SELECT ce.work_id FROM copies c JOIN editions ce ON ce.id = c.edition_id WHERE c.id = OLD.copy_id);
END;

CREATE TRIGGER course_copies_changed_insert AFTER INSERT ON course_copies BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e
    WHERE e.work_id = (SELECT ce.work_id FROM copies c JOIN editions ce ON ce.id = c.edition_id WHERE c.id = NEW.copy_id);
END;

CREATE TRIGGER declined_suggestions_changed_insert AFTER INSERT ON declined_suggestions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER declined_suggestions_changed_update AFTER UPDATE ON declined_suggestions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER document_files_changed_delete AFTER DELETE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.document_id);
END;

CREATE TRIGGER document_files_changed_insert AFTER INSERT ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;

CREATE TRIGGER document_files_changed_update AFTER UPDATE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;

CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;

CREATE TRIGGER documents_changed_insert AFTER INSERT ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;

CREATE TRIGGER documents_changed_update AFTER UPDATE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;

CREATE TRIGGER editions_changed_delete AFTER DELETE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.id);
END;

CREATE TRIGGER editions_changed_insert AFTER INSERT ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;

CREATE TRIGGER editions_changed_update AFTER UPDATE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;

CREATE TRIGGER enrichment_changed_delete AFTER DELETE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', OLD.edition_id);
END;

CREATE TRIGGER enrichment_changed_insert AFTER INSERT ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;

CREATE TRIGGER enrichment_changed_update AFTER UPDATE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;

CREATE TRIGGER label_queue_changed_delete AFTER DELETE ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', OLD.copy_id);
END;

CREATE TRIGGER label_queue_changed_insert AFTER INSERT ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', NEW.copy_id);
END;

CREATE TRIGGER loans_changed_delete AFTER DELETE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', OLD.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = OLD.copy_id;
END;

CREATE TRIGGER loans_changed_insert AFTER INSERT ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;

CREATE TRIGGER loans_changed_update AFTER UPDATE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;

CREATE TRIGGER lookup_proposals_changed_insert AFTER INSERT ON lookup_proposals BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER lookup_proposals_changed_update AFTER UPDATE ON lookup_proposals BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;

CREATE TRIGGER readings_changed_delete AFTER DELETE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = OLD.work_id;
END;

CREATE TRIGGER readings_changed_insert AFTER INSERT ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;

CREATE TRIGGER readings_changed_update AFTER UPDATE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;

CREATE TRIGGER scans_changed_delete AFTER DELETE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', OLD.id);
END;

CREATE TRIGGER scans_changed_insert AFTER INSERT ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;

CREATE TRIGGER scans_changed_update AFTER UPDATE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;

CREATE TRIGGER series_changed AFTER UPDATE ON series BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e JOIN works w ON w.id = e.work_id WHERE w.series_id = NEW.id;
END;

CREATE TRIGGER works_changed AFTER UPDATE ON works BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.id;
END;
