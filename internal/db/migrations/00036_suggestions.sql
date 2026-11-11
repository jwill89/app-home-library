-- +goose Up

-- What the sources proposed for an edition the last time the owner had it looked up, each
-- source asked again, and the details the owner declined. What a lookup offers to
-- change is worked out from these and the book as it is when read: a detail the owner has
-- since taken or typed is no longer offered, and a declined value is not offered again while
-- the sources still give it.
CREATE TABLE lookup_proposals (
    edition_id INTEGER PRIMARY KEY REFERENCES editions (id) ON DELETE CASCADE,
    proposal TEXT NOT NULL
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

-- A page showing what the sources offer follows a new proposal and a decline, on any device.
-- +goose StatementBegin
CREATE TRIGGER lookup_proposals_changed_insert AFTER INSERT ON lookup_proposals BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER lookup_proposals_changed_update AFTER UPDATE ON lookup_proposals BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER declined_suggestions_changed_insert AFTER INSERT ON declined_suggestions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER declined_suggestions_changed_update AFTER UPDATE ON declined_suggestions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd

-- +goose Down
DROP TABLE declined_suggestions;
DROP TABLE lookup_proposals;
