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

-- name: UpdateUserPhonkLevel :one
UPDATE users
SET phonk_level = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

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

-- name: BanUser :exec
UPDATE users 
SET is_banned = true, banned_at = NOW(), ban_reason = $2
WHERE id = $1;

-- name: UnbanUser :exec
UPDATE users 
SET is_banned = false, banned_at = NULL, ban_reason = NULL
WHERE id = $1;

-- name: UpdateFCMToken :exec
UPDATE users SET fcm_token = $2 WHERE id = $1;

-- name: GetUserFCMTokens :many
SELECT fcm_token FROM users
WHERE is_banned = false AND fcm_token IS NOT NULL AND fcm_token != '';

-- name: CountUsers :one
SELECT COUNT(*) FROM users;

-- name: UpdateAvatarURL :one
UPDATE users
SET avatar_url = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateUsername :one
UPDATE users
SET username = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;
