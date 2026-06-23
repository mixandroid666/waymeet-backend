-- Auth queries: lookups, pending-user creation, and OTP lifecycle.
-- Casts (::text) force non-null Go param types against nullable columns.

-- name: FindUserByEmail :one
SELECT id, status FROM users WHERE email = sqlc.arg(email)::text;

-- name: FindUserByPhone :one
SELECT id, status FROM users WHERE phone = sqlc.arg(phone)::text;

-- name: CreatePendingUserByEmail :one
INSERT INTO users (email, password_hash, status)
VALUES (sqlc.arg(email)::text, sqlc.arg(password_hash)::text, 'pending_verification')
RETURNING id, status;

-- name: CreatePendingUserByPhone :one
INSERT INTO users (phone, password_hash, status)
VALUES (sqlc.arg(phone)::text, sqlc.arg(password_hash)::text, 'pending_verification')
RETURNING id, status;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = sqlc.arg(password_hash)::text, updated_at = now()
WHERE id = sqlc.arg(id);

-- name: GetCredentialsByID :one
SELECT id, status, password_hash FROM users WHERE id = sqlc.arg(id);

-- name: ActivateUser :exec
UPDATE users SET status = 'active', verified_at = now(), updated_at = now()
WHERE id = sqlc.arg(id);

-- name: CreateOTP :one
INSERT INTO otp_codes (user_id, purpose, code_hash, expires_at)
VALUES (sqlc.arg(user_id), sqlc.arg(purpose), sqlc.arg(code_hash), sqlc.arg(expires_at))
RETURNING id, expires_at;

-- name: GetLatestOTP :one
SELECT id, code_hash, attempts, expires_at, created_at
FROM otp_codes
WHERE user_id = sqlc.arg(user_id) AND purpose = sqlc.arg(purpose) AND consumed_at IS NULL
ORDER BY created_at DESC
LIMIT 1;

-- name: IncrementOTPAttempts :exec
UPDATE otp_codes SET attempts = attempts + 1 WHERE id = sqlc.arg(id);

-- name: ConsumeOTP :exec
UPDATE otp_codes SET consumed_at = now() WHERE id = sqlc.arg(id);

-- name: InvalidatePendingOTPs :exec
UPDATE otp_codes SET consumed_at = now()
WHERE user_id = sqlc.arg(user_id) AND purpose = sqlc.arg(purpose) AND consumed_at IS NULL;
