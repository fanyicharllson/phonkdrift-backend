-- name: CreateFeedback :one
INSERT INTO feedback (user_id, rating, comment, app_version)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListFeedbackAdmin :many
SELECT f.*, u.username, u.email
FROM feedback f
JOIN users u ON f.user_id = u.id
ORDER BY f.created_at DESC
LIMIT $1 OFFSET ($2::int * $1);

-- name: CountFeedback :one
SELECT COUNT(*) FROM feedback;
