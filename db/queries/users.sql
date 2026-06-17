-- name: CreateUser :one
INSERT INTO users (email, password_hash, display_name)
VALUES ($1, $2, $3)
RETURNING id, email, display_name, avatar_url, bio, gender, birth_date, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, avatar_url, bio, gender, birth_date, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, display_name, avatar_url, bio, gender, birth_date, created_at
FROM users
WHERE id = $1;

-- name: UpdateUserLocation :exec
UPDATE users
SET location = ST_SetSRID(ST_MakePoint(sqlc.arg(lng)::float8, sqlc.arg(lat)::float8), 4326)::geography,
    updated_at = now()
WHERE id = sqlc.arg(id);
