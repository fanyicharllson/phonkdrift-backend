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