-- name: CreateUser :one
INSERT INTO users (
    username, email, password_hash, verification_code, code_expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, username, email, avatar_url, is_verified, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1::text LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1::uuid LIMIT 1;

-- name: GetVerificationDetails :one
SELECT id, verification_code, code_expires_at, is_verified
FROM users
WHERE email = $1::text LIMIT 1;

-- name: UpdateUserVerificationCode :exec
UPDATE users
SET verification_code = @verification_code::text,
    code_expires_at = @code_expires_at::timestamptz,
    updated_at = CURRENT_TIMESTAMP
WHERE email = @email::text;

-- name: UpdateUserVerification :one
UPDATE users
SET is_verified = @is_verified::boolean, 
    verification_code = NULL, 
    code_expires_at = NULL, 
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id::uuid
RETURNING *;

-- name: UpdatePassword :exec
UPDATE users
SET password_hash = @password_hash::text,
    verification_code = NULL,
    code_expires_at = NULL,
    updated_at = CURRENT_TIMESTAMP
WHERE id = @id::uuid;