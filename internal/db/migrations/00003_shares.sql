-- +goose Up

-- Read-only share links. A link is a secret address; only its hash is kept, so the owner sees
-- it once, when it is made. Removing the row revokes it at once: every visitor's request
-- checks that its link is still here and still running.
CREATE TABLE shares (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE shares;
