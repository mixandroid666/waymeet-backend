-- name: UpsertDeviceToken :exec
INSERT INTO devices (user_id, fcm_token, platform)
VALUES ($1, $2, $3)
ON CONFLICT (fcm_token) DO UPDATE
    SET user_id  = EXCLUDED.user_id,
        platform = EXCLUDED.platform;

-- name: GetDeviceTokensByUser :many
SELECT fcm_token FROM devices WHERE user_id = $1;

-- name: DeleteDeviceToken :exec
DELETE FROM devices WHERE fcm_token = $1;
