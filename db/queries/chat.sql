-- name: GetOrCreateConversation :one
INSERT INTO conversations (user_a, user_b)
VALUES (
    LEAST(sqlc.arg(user_a)::uuid,    sqlc.arg(user_b)::uuid),
    GREATEST(sqlc.arg(user_a)::uuid, sqlc.arg(user_b)::uuid)
)
ON CONFLICT (user_a, user_b) DO UPDATE SET id = conversations.id
RETURNING id, user_a, user_b, created_at;

-- name: GetConversationPartner :one
SELECT
    CASE WHEN user_a = sqlc.arg(viewer_id) THEN user_b ELSE user_a END AS partner_id
FROM conversations
WHERE id = sqlc.arg(conversation_id)
  AND (user_a = sqlc.arg(viewer_id) OR user_b = sqlc.arg(viewer_id));

-- name: ListConversations :many
SELECT
    c.id,
    CASE WHEN c.user_a = sqlc.arg(viewer_id) THEN c.user_b ELSE c.user_a END AS partner_id,
    u.display_name  AS partner_name,
    u.avatar_url    AS partner_avatar_url,
    m.id            AS last_message_id,
    m.body          AS last_message,
    m.sender_id     AS last_sender_id,
    m.created_at    AS last_message_at
FROM conversations c
JOIN users u ON u.id = CASE WHEN c.user_a = sqlc.arg(viewer_id) THEN c.user_b ELSE c.user_a END
LEFT JOIN LATERAL (
    SELECT id, body, sender_id, created_at
    FROM messages
    WHERE conversation_id = c.id
    ORDER BY created_at DESC
    LIMIT 1
) m ON true
WHERE c.user_a = sqlc.arg(viewer_id) OR c.user_b = sqlc.arg(viewer_id)
ORDER BY COALESCE(m.created_at, c.created_at) DESC;

-- name: ListMessages :many
SELECT id, conversation_id, sender_id, body, msg_type, media_url, created_at
FROM messages
WHERE conversation_id = sqlc.arg(conversation_id)
ORDER BY created_at ASC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CreateMessage :one
INSERT INTO messages (conversation_id, sender_id, body, msg_type, media_url)
VALUES (sqlc.arg(conversation_id), sqlc.arg(sender_id), sqlc.arg(body), sqlc.arg(msg_type), sqlc.arg(media_url))
RETURNING id, conversation_id, sender_id, body, msg_type, media_url, created_at;

-- name: GetConversationPartners :many
SELECT
    CASE WHEN user_a = sqlc.arg(user_id) THEN user_b ELSE user_a END AS partner_id
FROM conversations
WHERE user_a = sqlc.arg(user_id) OR user_b = sqlc.arg(user_id);
