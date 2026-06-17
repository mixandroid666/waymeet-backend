-- name: CreatePost :one
INSERT INTO posts (author_id, body)
VALUES ($1, $2)
RETURNING id, author_id, body, created_at;

-- name: AddPostImage :exec
INSERT INTO post_images (post_id, url, position)
VALUES ($1, $2, $3);

-- name: ListHomeTimeline :many
-- Fan-out-on-read timeline: posts by the viewer and everyone they follow.
SELECT
    p.id,
    p.author_id,
    u.display_name AS author_name,
    u.avatar_url   AS author_avatar_url,
    p.body,
    p.created_at,
    (SELECT count(*) FROM post_likes l WHERE l.post_id = p.id)  AS like_count,
    (SELECT count(*) FROM comments c   WHERE c.post_id = p.id)  AS comment_count
FROM posts p
JOIN users u ON u.id = p.author_id
WHERE p.author_id = sqlc.arg(viewer_id)
   OR p.author_id IN (
        SELECT followee_id FROM follows WHERE follower_id = sqlc.arg(viewer_id)
   )
ORDER BY p.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: LikePost :exec
INSERT INTO post_likes (post_id, user_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;
