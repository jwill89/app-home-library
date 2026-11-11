-- +goose Up

-- What changed, for the change feed: a row a change, written by triggers in the
-- transaction that makes it, so a write from any process is in it, the web server's, the text
-- service's, or an import run by hand, and none can be forgotten. The web server reads it and
-- streams each change to the owner's open pages, which fetch what they show again. The ids
-- only ever rise (AUTOINCREMENT), even once old rows are let go, so a page's cursor holds.
CREATE TABLE changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL CHECK (type IN ('book', 'scan', 'document', 'lookup', 'loan', 'label')),
    ref INTEGER NOT NULL,
    -- When, so a new stream can start a few seconds back (events.Feed.Lookback).
    at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

-- A book is an edition: its row, its work's, its copies', its reads', and its series' name.
-- +goose StatementBegin
CREATE TRIGGER editions_changed_insert AFTER INSERT ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_update AFTER UPDATE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER editions_changed_delete AFTER DELETE ON editions BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER works_changed AFTER UPDATE ON works BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER series_changed AFTER UPDATE ON series BEGIN
    INSERT INTO changes (type, ref)
    SELECT 'book', e.id FROM editions e JOIN works w ON w.id = e.work_id WHERE w.series_id = NEW.id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_insert AFTER INSERT ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_update AFTER UPDATE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER copies_changed_delete AFTER DELETE ON copies BEGIN
    INSERT INTO changes (type, ref) VALUES ('book', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_insert AFTER INSERT ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_update AFTER UPDATE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = NEW.work_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER readings_changed_delete AFTER DELETE ON readings BEGIN
    INSERT INTO changes (type, ref) SELECT 'book', e.id FROM editions e WHERE e.work_id = OLD.work_id;
END;
-- +goose StatementEnd

-- A loan is its own, and its copy's book's: the library shows a copy out.
-- +goose StatementBegin
CREATE TRIGGER loans_changed_insert AFTER INSERT ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_update AFTER UPDATE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', NEW.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = NEW.copy_id;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER loans_changed_delete AFTER DELETE ON loans BEGIN
    INSERT INTO changes (type, ref) VALUES ('loan', OLD.id);
    INSERT INTO changes (type, ref) SELECT 'book', c.edition_id FROM copies c WHERE c.id = OLD.copy_id;
END;
-- +goose StatementEnd

-- The background's work: a scan looked up, a book's lookup queued or done, a document's file
-- read by the text service, a label queued or printed.
-- +goose StatementBegin
CREATE TRIGGER scans_changed_insert AFTER INSERT ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_update AFTER UPDATE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER scans_changed_delete AFTER DELETE ON scans BEGIN
    INSERT INTO changes (type, ref) VALUES ('scan', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_insert AFTER INSERT ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_update AFTER UPDATE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', NEW.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER enrichment_changed_delete AFTER DELETE ON enrichment BEGIN
    INSERT INTO changes (type, ref) VALUES ('lookup', OLD.edition_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_insert AFTER INSERT ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_update AFTER UPDATE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER documents_changed_delete AFTER DELETE ON documents BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_insert AFTER INSERT ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_update AFTER UPDATE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', NEW.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER document_files_changed_delete AFTER DELETE ON document_files BEGIN
    INSERT INTO changes (type, ref) VALUES ('document', OLD.document_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_insert AFTER INSERT ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', NEW.copy_id);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER label_queue_changed_delete AFTER DELETE ON label_queue BEGIN
    INSERT INTO changes (type, ref) VALUES ('label', OLD.copy_id);
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER label_queue_changed_delete;
DROP TRIGGER label_queue_changed_insert;
DROP TRIGGER document_files_changed_delete;
DROP TRIGGER document_files_changed_update;
DROP TRIGGER document_files_changed_insert;
DROP TRIGGER documents_changed_delete;
DROP TRIGGER documents_changed_update;
DROP TRIGGER documents_changed_insert;
DROP TRIGGER enrichment_changed_delete;
DROP TRIGGER enrichment_changed_update;
DROP TRIGGER enrichment_changed_insert;
DROP TRIGGER scans_changed_delete;
DROP TRIGGER scans_changed_update;
DROP TRIGGER scans_changed_insert;
DROP TRIGGER loans_changed_delete;
DROP TRIGGER loans_changed_update;
DROP TRIGGER loans_changed_insert;
DROP TRIGGER readings_changed_delete;
DROP TRIGGER readings_changed_update;
DROP TRIGGER readings_changed_insert;
DROP TRIGGER copies_changed_delete;
DROP TRIGGER copies_changed_update;
DROP TRIGGER copies_changed_insert;
DROP TRIGGER series_changed;
DROP TRIGGER works_changed;
DROP TRIGGER editions_changed_delete;
DROP TRIGGER editions_changed_update;
DROP TRIGGER editions_changed_insert;
DROP TABLE changes;
