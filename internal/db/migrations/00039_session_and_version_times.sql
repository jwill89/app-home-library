-- +goose NO TRANSACTION
-- +goose Up

-- The two tables 00038 left in their libraries' forms take the one form too: a session's
-- expiry, kept by the app's own session store now, becomes a DATETIME where it was a Julian
-- day (every session still running keeps running), and goose's record of the migrations
-- takes the same default and CHECK, which goose leaves its timestamps to. The same rebuild as
-- 00038's, with its checks.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_dates_and_references_all_kept CHECK (ok)
);
CREATE TEMP TABLE kept_sequence AS
SELECT name, seq FROM sqlite_sequence WHERE name = 'goose_db_version';

CREATE TABLE sessions_new (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry DATETIME NOT NULL
        CHECK (expiry IS strftime('%Y-%m-%dT%H:%M:%SZ', expiry))
);
INSERT INTO sessions_new (token, data, expiry)
SELECT token, data, strftime('%Y-%m-%dT%H:%M:%SZ', expiry)
FROM sessions;
INSERT INTO rebuild_checks SELECT 'sessions rows', (SELECT count(*) FROM sessions) = (SELECT count(*) FROM sessions_new);
DROP TABLE sessions;
ALTER TABLE sessions_new RENAME TO sessions;
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE goose_db_version_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id INTEGER NOT NULL,
    is_applied INTEGER NOT NULL,
    tstamp DATETIME DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
        CHECK (tstamp IS strftime('%Y-%m-%dT%H:%M:%SZ', tstamp))
);
INSERT INTO goose_db_version_new (id, version_id, is_applied, tstamp)
SELECT id, version_id, is_applied, strftime('%Y-%m-%dT%H:%M:%SZ', tstamp)
FROM goose_db_version;
INSERT INTO rebuild_checks SELECT 'goose_db_version rows', (SELECT count(*) FROM goose_db_version) = (SELECT count(*) FROM goose_db_version_new);
INSERT INTO rebuild_checks SELECT 'goose_db_version.tstamp', (SELECT count(*) FROM goose_db_version WHERE tstamp IS NOT NULL) = (SELECT count(*) FROM goose_db_version_new WHERE tstamp IS NOT NULL);
DROP TABLE goose_db_version;
ALTER TABLE goose_db_version_new RENAME TO goose_db_version;

INSERT INTO sqlite_sequence (name, seq)
SELECT k.name, k.seq FROM temp.kept_sequence k
WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence s WHERE s.name = k.name);
UPDATE sqlite_sequence
SET seq = max(seq, (SELECT k.seq FROM temp.kept_sequence k WHERE k.name = sqlite_sequence.name))
WHERE name IN (SELECT name FROM temp.kept_sequence);

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
DROP TABLE temp.kept_sequence;
PRAGMA user_version = 39;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;

-- +goose Down
-- Each table as it was: the session's expiry a Julian day, goose's timestamps SQLite's own
-- space-separated form.
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;
BEGIN IMMEDIATE;

CREATE TEMP TABLE rebuild_checks (
    name TEXT NOT NULL,
    ok INTEGER NOT NULL,
    CONSTRAINT rows_and_references_all_kept CHECK (ok)
);
CREATE TEMP TABLE kept_sequence AS
SELECT name, seq FROM sqlite_sequence WHERE name = 'goose_db_version';

CREATE TABLE sessions_old (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    expiry REAL NOT NULL
);
INSERT INTO sessions_old (token, data, expiry)
SELECT token, data, julianday(expiry)
FROM sessions;
INSERT INTO rebuild_checks SELECT 'sessions rows', (SELECT count(*) FROM sessions) = (SELECT count(*) FROM sessions_old);
DROP TABLE sessions;
ALTER TABLE sessions_old RENAME TO sessions;
CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE goose_db_version_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version_id INTEGER NOT NULL,
    is_applied INTEGER NOT NULL,
    tstamp TIMESTAMP DEFAULT (datetime('now'))
);
INSERT INTO goose_db_version_old (id, version_id, is_applied, tstamp)
SELECT id, version_id, is_applied, datetime(tstamp)
FROM goose_db_version;
INSERT INTO rebuild_checks SELECT 'goose_db_version rows', (SELECT count(*) FROM goose_db_version) = (SELECT count(*) FROM goose_db_version_old);
DROP TABLE goose_db_version;
ALTER TABLE goose_db_version_old RENAME TO goose_db_version;

INSERT INTO sqlite_sequence (name, seq)
SELECT k.name, k.seq FROM temp.kept_sequence k
WHERE NOT EXISTS (SELECT 1 FROM sqlite_sequence s WHERE s.name = k.name);
UPDATE sqlite_sequence
SET seq = max(seq, (SELECT k.seq FROM temp.kept_sequence k WHERE k.name = sqlite_sequence.name))
WHERE name IN (SELECT name FROM temp.kept_sequence);

INSERT INTO rebuild_checks SELECT 'foreign keys', NOT EXISTS (SELECT 1 FROM pragma_foreign_key_check);
DROP TABLE temp.rebuild_checks;
DROP TABLE temp.kept_sequence;
PRAGMA user_version = 38;
COMMIT;
PRAGMA legacy_alter_table = OFF;
PRAGMA foreign_keys = ON;
