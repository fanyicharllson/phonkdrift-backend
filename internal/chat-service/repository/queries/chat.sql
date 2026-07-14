-- name: JoinCommunity :one
INSERT INTO community_members (user_id, username, avatar_url) VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO NOTHING
RETURNING user_id;

-- name: IsCommunityMember :one
SELECT EXISTS(SELECT 1 FROM community_members WHERE user_id = $1) AS is_member;

-- name: CountCommunityMembers :one
SELECT COUNT(*) FROM community_members;

-- name: ListCommunityMembers :many
SELECT user_id, username, avatar_url, joined_at,
       ROW_NUMBER() OVER (ORDER BY joined_at ASC) AS join_rank
FROM community_members
ORDER BY joined_at ASC
LIMIT $1 OFFSET $2;

-- name: CreateMessage :one
INSERT INTO chat_messages (user_id, username, avatar_url, content, media_url, message_type, reply_to_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetMessageByID :one
SELECT * FROM chat_messages WHERE id = $1 LIMIT 1;

-- name: GetMessagesBefore :many
SELECT * FROM chat_messages
WHERE created_at < $1
ORDER BY created_at DESC
LIMIT $2;
