package dbgen

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

const upsertDeviceToken = `-- name: UpsertDeviceToken :exec
INSERT INTO devices (user_id, fcm_token, platform)
VALUES ($1, $2, $3)
ON CONFLICT (fcm_token) DO UPDATE
    SET user_id  = EXCLUDED.user_id,
        platform = EXCLUDED.platform`

type UpsertDeviceTokenParams struct {
	UserID   pgtype.UUID `json:"user_id"`
	FcmToken string      `json:"fcm_token"`
	Platform string      `json:"platform"`
}

func (q *Queries) UpsertDeviceToken(ctx context.Context, arg UpsertDeviceTokenParams) error {
	_, err := q.db.Exec(ctx, upsertDeviceToken, arg.UserID, arg.FcmToken, arg.Platform)
	return err
}

const getDeviceTokensByUser = `-- name: GetDeviceTokensByUser :many
SELECT fcm_token FROM devices WHERE user_id = $1`

func (q *Queries) GetDeviceTokensByUser(ctx context.Context, userID pgtype.UUID) ([]string, error) {
	rows, err := q.db.Query(ctx, getDeviceTokensByUser, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

const deleteDeviceToken = `-- name: DeleteDeviceToken :exec
DELETE FROM devices WHERE fcm_token = $1`

func (q *Queries) DeleteDeviceToken(ctx context.Context, token string) error {
	_, err := q.db.Exec(ctx, deleteDeviceToken, token)
	return err
}
