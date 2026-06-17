-- Login + session queries.

-- name: GetCredentialsByEmail :one
SELECT id, status, password_hash FROM users WHERE email = sqlc.arg(email)::text;

-- name: GetCredentialsByPhone :one
SELECT id, status, password_hash FROM users WHERE phone = sqlc.arg(phone)::text;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
VALUES (sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(expires_at))
RETURNING id, expires_at;

-- name: GetRefreshToken :one
SELECT id, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash);

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE id = sqlc.arg(id);

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now()
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL;
