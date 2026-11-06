-- +goose Up

-- The calendar link: a secret address for a feed of the loans' due dates, which a phone's
-- calendar subscribes to and reminds from. Only its hash is kept, as a share link's is. There
-- is one at most; making another replaces it.
CREATE TABLE calendar_links (
    id INTEGER PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    last_used_at TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE calendar_links;
