-- name: CreateUser :one
INSERT INTO users (
    username, email, password_hash
) VALUES (
    $1, $2, $3
)
RETURNING id, username, email, avatar_url, is_verified, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1::text LIMIT 1;

-- name: UpdateUserVerification :one
UPDATE users
SET is_verified = $1::boolean, updated_at = CURRENT_TIMESTAMP
WHERE id = $2::uuid
RETURNING *;