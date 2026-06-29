-- name: GetTrack :one
SELECT * FROM tracks WHERE id = $1 LIMIT 1;

-- name: UpdateTrackStats :exec
UPDATE tracks 
SET play_count = play_count + @play_count_change::integer, 
    likes_count = likes_count + @likes_count_change::integer
WHERE id = @track_id;

-- name: SetTrackInteraction :exec
INSERT INTO track_interactions (user_id, track_id, is_liked, interacted_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (user_id, track_id) 
DO UPDATE SET is_liked = EXCLUDED.is_liked, interacted_at = now();

-- name: DeleteTrackInteraction :exec
DELETE FROM track_interactions 
WHERE user_id = $1 AND track_id = $2;

-- name: SyncPlaybackTelemetry :exec
INSERT INTO listening_history (user_id, track_id, last_position_sec, is_completed, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (user_id, track_id) 
DO UPDATE SET last_position_sec = EXCLUDED.last_position_sec, is_completed = EXCLUDED.is_completed, updated_at = now();

-- name: GetRecentlyPlayed :many
SELECT t.*, lh.last_position_sec, lh.updated_at
FROM listening_history lh
JOIN tracks t ON lh.track_id = t.id
WHERE lh.user_id = $1
ORDER BY lh.updated_at DESC
LIMIT $2;

-- name: GetTrendingTracks :many
SELECT * FROM tracks
ORDER BY play_count DESC, likes_count DESC
LIMIT $1;

-- name: GetTrackByYoutubeID :one
SELECT * FROM tracks WHERE youtube_id = $1 LIMIT 1;

-- name: InsertTrack :one
INSERT INTO tracks (
  id, title, artist_id, artist_name, duration,
  thumbnail_url, youtube_id, storage_url, genre,
  source, is_approved, yt_view_count
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: ApproveTrack :exec
UPDATE tracks SET is_approved = true, is_rejected = false WHERE id = $1;

-- name: RejectTrack :exec
UPDATE tracks SET is_rejected = true, is_approved = false WHERE id = $1;

-- name: ToggleFeatureTrack :exec
UPDATE tracks SET is_featured = $2 WHERE id = $1;

-- name: DeleteTrack :exec
DELETE FROM tracks WHERE id = $1;

-- name: GetAllTracksAdmin :many
SELECT * FROM tracks ORDER BY created_at DESC LIMIT $1 OFFSET $2;

-- name: MarkTrackFCMNotified :exec
UPDATE tracks SET fcm_notified = true WHERE id = $1;

-- name: GetApprovedUnnotifiedTracks :many
SELECT * FROM tracks 
WHERE is_approved = true AND fcm_notified = false
ORDER BY created_at DESC;

-- name: SearchTracks :many
SELECT * FROM tracks
WHERE is_approved = true 
  AND is_rejected = false
  AND storage_url IS NOT NULL
  AND to_tsvector('english', title || ' ' || artist_name) @@ plainto_tsquery('english', $1)
ORDER BY play_count DESC
LIMIT 20 OFFSET ($2 * 20);

-- name: GetForYouTracks :many
SELECT * FROM tracks
WHERE is_approved = true AND is_rejected = false AND storage_url IS NOT NULL
ORDER BY RANDOM()
LIMIT $1;