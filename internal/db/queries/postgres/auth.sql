-- name: CreateAccount :one
INSERT INTO auth_accounts (username, password_hash, role)
VALUES ($1, $2, $3)
RETURNING id, username, role, banned, created_at;

-- name: GetAccountByUsername :one
SELECT id, username, password_hash, role, banned, created_at, updated_at
FROM auth_accounts WHERE username = $1;

-- name: GetAccountByID :one
SELECT id, username, password_hash, role, banned, created_at, updated_at
FROM auth_accounts WHERE id = $1;

-- name: DeleteAccount :exec
DELETE FROM auth_accounts WHERE username = $1;

-- name: UpdatePassword :exec
UPDATE auth_accounts SET password_hash = $1, updated_at = now() WHERE username = $2;

-- name: SetBanned :exec
UPDATE auth_accounts SET banned = $1, updated_at = now() WHERE username = $2;

-- name: ListAccounts :many
SELECT id, username, role, banned, created_at FROM auth_accounts ORDER BY username;

-- name: CreateSession :one
INSERT INTO auth_sessions (account_id, token, expires_at)
VALUES ($1, $2, $3)
RETURNING id, token, expires_at;

-- name: GetSession :one
SELECT s.id, s.account_id, s.token, s.expires_at, a.username, a.role, a.banned
FROM auth_sessions s
JOIN auth_accounts a ON a.id = s.account_id
WHERE s.token = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM auth_sessions WHERE token = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM auth_sessions WHERE expires_at <= now();

-- name: TouchSession :exec
UPDATE auth_sessions SET expires_at = $1 WHERE token = $2;
