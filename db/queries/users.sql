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

-- name: UpdateProfile :one
-- Partial profile update: each field is left unchanged when its arg is null
-- (COALESCE), so this serves both first-time setup and later edits.
UPDATE users SET
    display_name = COALESCE(sqlc.narg(display_name), display_name),
    birth_date   = COALESCE(sqlc.narg(birth_date),   birth_date),
    gender       = COALESCE(sqlc.narg(gender),       gender),
    bio          = COALESCE(sqlc.narg(bio),          bio),
    avatar_url   = COALESCE(sqlc.narg(avatar_url),   avatar_url),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING id, email, phone, display_name, avatar_url, bio, gender, birth_date, status, created_at;

-- name: GetProfile :one
SELECT id, email, phone, display_name, avatar_url, bio, gender, birth_date, status, created_at
FROM users
WHERE id = sqlc.arg(id);

-- name: CountFollowers :one
SELECT count(*) FROM follows WHERE followee_id = sqlc.arg(user_id);

-- name: CountFollowing :one
SELECT count(*) FROM follows WHERE follower_id = sqlc.arg(user_id);

-- name: ListProfilePhotos :many
SELECT photo_url FROM profile_photos
WHERE user_id = sqlc.arg(user_id)
ORDER BY photo_order;

-- name: DeleteProfilePhotos :exec
DELETE FROM profile_photos WHERE user_id = sqlc.arg(user_id);

-- name: InsertProfilePhoto :exec
INSERT INTO profile_photos (user_id, photo_url, photo_order)
VALUES (sqlc.arg(user_id), sqlc.arg(photo_url), sqlc.arg(photo_order)::smallint);

-- name: GetPublicProfile :one
SELECT
    u.id,
    u.display_name,
    u.avatar_url,
    u.bio,
    u.gender,
    date_part('year', age(u.birth_date))::int AS age,
    (SELECT count(*) FROM follows WHERE followee_id = u.id)   AS follower_count,
    (SELECT count(*) FROM follows WHERE follower_id = u.id)   AS following_count,
    EXISTS (
        SELECT 1 FROM follows
        WHERE follower_id = sqlc.arg(viewer_id) AND followee_id = u.id
    ) AS is_following
FROM users u
WHERE u.id = sqlc.arg(target_id);

-- name: FollowUser :exec
INSERT INTO follows (follower_id, followee_id)
VALUES (sqlc.arg(follower_id), sqlc.arg(followee_id))
ON CONFLICT DO NOTHING;

-- name: UnfollowUser :exec
DELETE FROM follows
WHERE follower_id = sqlc.arg(follower_id) AND followee_id = sqlc.arg(followee_id);
