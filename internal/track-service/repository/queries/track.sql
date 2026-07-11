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

-- name: MarkTracksFCMNotified :exec
UPDATE tracks SET fcm_notified = true WHERE id = ANY(@ids::text[]);

-- name: GetApprovedUnnotifiedTracks :many
SELECT * FROM tracks 
WHERE is_approved = true AND fcm_notified = false
ORDER BY created_at DESC;

-- name: SearchTracks :many
SELECT * FROM tracks
WHERE is_approved = true 
  AND is_rejected = false
  AND storage_url IS NOT NULL
  AND to_tsvector('english', title || ' ' || artist_name) @@ plainto_tsquery('english', $1::text)
ORDER BY play_count DESC
LIMIT 20 OFFSET ($2::integer * 20);

-- name: GetForYouTracks :many
SELECT * FROM tracks
WHERE is_approved = true AND is_rejected = false AND storage_url IS NOT NULL
ORDER BY RANDOM()
LIMIT $1;

-- name: GetTrendingTracks :many
SELECT * FROM tracks
WHERE is_approved = true AND is_rejected = false AND storage_url IS NOT NULL
ORDER BY play_count DESC, likes_count DESC
LIMIT $1;

-- name: GetLikedTracks :many
SELECT t.* FROM tracks t
INNER JOIN track_interactions ti ON t.id = ti.track_id
WHERE ti.user_id = $1 AND ti.is_liked = true
ORDER BY ti.interacted_at DESC
LIMIT $2 OFFSET ($3::int * $2);

-- name: CreatePlaylist :one
INSERT INTO playlists (user_id, name, cover_image_url)
VALUES ($1, $2, $3)
RETURNING *;

-- name: AddTrackToPlaylist :exec
INSERT INTO playlist_tracks (playlist_id, track_id)
VALUES ($1, $2)
ON CONFLICT (playlist_id, track_id) DO NOTHING;

-- name: RemoveTrackFromPlaylist :exec
DELETE FROM playlist_tracks WHERE playlist_id = $1 AND track_id = $2;

-- name: GetPlaylistByID :one
SELECT * FROM playlists WHERE id = $1 LIMIT 1;

-- name: DeletePlaylist :exec
DELETE FROM playlists WHERE id = $1;

-- name: GetPlaylistTracks :many
SELECT t.* FROM tracks t
INNER JOIN playlist_tracks pt ON t.id = pt.track_id
WHERE pt.playlist_id = $1
ORDER BY pt.added_at DESC;

-- name: UpdateTrackStorageURL :exec
UPDATE tracks SET storage_url = $2 WHERE id = $1;

-- name: GetUserPlaylists :many
SELECT p.*, COUNT(pt.track_id) AS track_count
FROM playlists p
LEFT JOIN playlist_tracks pt ON p.id = pt.playlist_id
WHERE p.user_id = $1
GROUP BY p.id
ORDER BY p.created_at DESC;