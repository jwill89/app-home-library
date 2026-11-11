-- name: CreateUser :one
INSERT INTO users (name) VALUES (?) RETURNING *;

-- name: GetFirstUser :one
SELECT * FROM users ORDER BY id LIMIT 1;

-- name: GetUser :one
SELECT * FROM users WHERE id = ?;

-- name: CreatePasskey :one
-- The passkey as the owner's list shows it.
INSERT INTO passkeys (user_id, rp_id, credential_id, credential, name)
VALUES (?, ?, ?, ?, ?)
RETURNING id, rp_id, name, created_at, last_used_at;

-- name: ListPasskeys :many
SELECT id, rp_id, name, created_at, last_used_at FROM passkeys WHERE user_id = ? ORDER BY id;

-- name: ListPasskeyCredentials :many
SELECT credential FROM passkeys WHERE user_id = ? AND rp_id = ?;

-- name: UsePasskey :exec
UPDATE passkeys
SET credential = ?, last_used_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE credential_id = ?;

-- name: DeletePasskey :execrows
DELETE FROM passkeys WHERE id = ? AND user_id = ?;

-- name: CreateSetupCode :exec
INSERT INTO setup_codes (code_hash, user_id, expires_at) VALUES (?, ?, ?);

-- name: TakeSetupCode :one
DELETE FROM setup_codes WHERE code_hash = ? AND expires_at > ? RETURNING user_id;

-- name: DeleteExpiredSetupCodes :exec
DELETE FROM setup_codes WHERE expires_at <= ?;

-- name: CountUserPasskey :one
-- Whether the passkey a session signed in with is still the user's.
SELECT count(*) FROM passkeys WHERE user_id = ? AND credential_id = ?;
