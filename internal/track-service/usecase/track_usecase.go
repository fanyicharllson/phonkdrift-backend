package usecase

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository"
	db "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ErrPlaylistAccessDenied is returned when the requester doesn't own a
// playlist they're trying to view (while private) or modify.
var ErrPlaylistAccessDenied = errors.New("playlist access denied")

type TrackUsecase interface {
	GetAudioStream(ctx context.Context, youtubeID string) (string, int64, error)
	SyncTelemetry(ctx context.Context, userID string, trackID string, pos int32, completed bool) error
	SearchTracks(ctx context.Context, query string, page int32) ([]*trackpb.TrackMetadata, error)
	GetTrending(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error)
	GetRecentHistory(ctx context.Context, userID string, limit int32) ([]*trackpb.TrackMetadata, error)
	SetInteraction(ctx context.Context, userID string, trackID string, isLiked bool) error
	NewPlaylist(ctx context.Context, userID string, name string, coverURL string) (*trackpb.PlaylistResponse, error)
	AddTrackToPlaylist(ctx context.Context, playlistID string, trackID string, requesterID string) error
	RemoveTrackFromPlaylist(ctx context.Context, playlistID string, trackID string, requesterID string) error
	GetPlaylist(ctx context.Context, playlistID string, requesterID string) (*trackpb.GetPlaylistResponse, error)
	DeletePlaylist(ctx context.Context, playlistID string, requesterID string) error
	GetUserPlaylists(ctx context.Context, userID string) ([]*trackpb.PlaylistSummary, error)
	GetForYou(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error)
	SeedTrack(ctx context.Context, req *trackpb.SeedTrackRequest) (string, error)
	ListTracksAdmin(ctx context.Context, page, limit int32) ([]*trackpb.TrackMetadata, int32, error)
	ApproveTrack(ctx context.Context, trackID string) error
	RejectTrack(ctx context.Context, trackID string) error
	FeatureTrack(ctx context.Context, trackID string, featured bool) error
	DeleteTrack(ctx context.Context, trackID string) error
	GetAdminStats(ctx context.Context) (*trackpb.AdminStatsResponse, error)
	GetLikedTracks(ctx context.Context, userID string, limit int32, page int32) ([]*trackpb.TrackMetadata, error)
}

type trackUsecase struct {
	repo        db.Querier
	rdb         *redis.Client
	ytAPIKey    string
	cookiesPath string
	publisher   *repository.TrackEventPublisher
}

func NewTrackUsecase(repo db.Querier, rdb *redis.Client, ytAPIKey string, cookiesPath string, publisher *repository.TrackEventPublisher) TrackUsecase {
	return &trackUsecase{
		repo:        repo,
		rdb:         rdb,
		ytAPIKey:    ytAPIKey,
		cookiesPath: cookiesPath,
		publisher:   publisher,
	}
}

// ==========================================
// 🔍 1. TRACK STREAMING & DISCOVERY LOGIC
// ==========================================

func (u *trackUsecase) GetAudioStream(ctx context.Context, youtubeID string) (string, int64, error) {
	cacheKey := fmt.Sprintf("stream:%s", youtubeID)

	// ⚡ Redis cache check first
	cachedURL, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedURL != "" {
		ttl, _ := u.rdb.TTL(ctx, cacheKey).Result()
		return cachedURL, time.Now().Add(ttl).Unix(), nil
	}

	// ✅ Check DB first — if we have a permanent storage_url, use it
	track, err := u.repo.GetTrackByYoutubeID(ctx, youtubeID)
	if err == nil && track.StorageUrl.Valid && track.StorageUrl.String != "" {
		// Permanent CDN URL — cache for 24h, never expires on DO Spaces
		_ = u.rdb.Set(ctx, cacheKey, track.StorageUrl.String, 24*time.Hour).Err()
		return track.StorageUrl.String, time.Now().Add(24 * time.Hour).Unix(), nil
	}

	// 🐌 Fallback: yt-dlp for tracks not yet in storage (legacy path)
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", youtubeID)
	args := []string{
		"-f", "bestaudio",
		"-g",
		"--js-runtimes", "node",
		"--no-playlist",
	}
	if u.cookiesPath != "" {
		args = append(args, "--cookies", u.cookiesPath)
	}
	args = append(args, videoURL)

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("yt-dlp crashed: %s (%w)", stderr.String(), err)
	}

	streamURL := strings.TrimSpace(out.String())
	if streamURL == "" {
		return "", 0, fmt.Errorf("extracted streaming url is empty")
	}

	cacheTTL := 5 * time.Hour
	_ = u.rdb.Set(ctx, cacheKey, streamURL, cacheTTL).Err()
	return streamURL, time.Now().Add(cacheTTL).Unix(), nil
}

func (u *trackUsecase) SearchTracks(ctx context.Context, query string, page int32) ([]*trackpb.TrackMetadata, error) {
	cleanQuery := strings.TrimSpace(strings.ToLower(query))
	cacheKey := fmt.Sprintf("search:%s:p%d", cleanQuery, page)

	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedTracks []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cachedTracks) == nil {
			return cachedTracks, nil
		}
	}

	// Search our DB first
	dbTracks, err := u.repo.SearchTracks(ctx, db.SearchTracksParams{
		Column1: cleanQuery,
		Column2: page,
	})

	var results []*trackpb.TrackMetadata
	if err == nil {
		for _, t := range dbTracks {
			results = append(results, trackToProto(t))
		}
	}

	// If DB has fewer than 3 results, augment with YouTube search
	if len(results) < 3 && u.ytAPIKey != "" {
		ytTracks, err := u.searchYouTube(ctx, cleanQuery+" phonk")
		if err == nil {
			// Deduplicate — skip YT results already in our DB
			existingIDs := map[string]bool{}
			for _, r := range results {
				existingIDs[r.OriginalYoutubeId] = true
			}
			for _, yt := range ytTracks {
				if !existingIDs[yt.OriginalYoutubeId] {
					results = append(results, yt)
				}
			}
		}
	}

	if jsonBytes, err := json.Marshal(results); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 15*time.Minute).Err()
	}

	return results, nil
}

// searchYouTube queries YouTube Data API and returns TrackMetadata
// These tracks stream via yt-dlp fallback path (no storage_url)
func (u *trackUsecase) searchYouTube(ctx context.Context, query string) ([]*trackpb.TrackMetadata, error) {
	url := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/search?part=snippet&q=%s&type=video&videoCategoryId=10&maxResults=10&key=%s",
		strings.ReplaceAll(query, " ", "+"),
		u.ytAPIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ytResp struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ytResp); err != nil {
		return nil, err
	}

	var tracks []*trackpb.TrackMetadata
	for _, item := range ytResp.Items {
		if item.ID.VideoID == "" {
			continue
		}
		tracks = append(tracks, &trackpb.TrackMetadata{
			TrackId:           "yt-" + item.ID.VideoID,
			Title:             item.Snippet.Title,
			ArtistName:        item.Snippet.ChannelTitle,
			Duration:          "00:00", // unknown until played
			ThumbnailUrl:      item.Snippet.Thumbnails.High.URL,
			OriginalYoutubeId: item.ID.VideoID,
			StorageUrl:        "", // empty = yt-dlp fallback path
			Source:            "youtube_search",
		})
	}

	return tracks, nil
}

// ensureTrackPersisted makes sure trackID exists as a real row in the tracks
// table before a favorite/playlist-add tries to reference it. Ephemeral
// YouTube search results carry a "yt-<videoID>" trackID that's never been
// persisted — inserting one here (idempotently) is what fixes the FK
// violation that previously surfaced as "could not add to playlist".
func (u *trackUsecase) ensureTrackPersisted(ctx context.Context, trackID string) error {
	const ytPrefix = "yt-"
	if !strings.HasPrefix(trackID, ytPrefix) {
		return nil // native DB track, already persisted
	}
	youtubeID := strings.TrimPrefix(trackID, ytPrefix)

	if _, err := u.repo.GetTrack(ctx, trackID); err == nil {
		return nil // already persisted (e.g. a previous favorite/playlist-add did this)
	}

	meta, err := u.fetchVideoMetadataByID(ctx, youtubeID)
	if err != nil {
		return fmt.Errorf("failed to fetch video metadata: %w", err)
	}

	_, err = u.repo.InsertTrack(ctx, db.InsertTrackParams{
		ID:           trackID,
		Title:        meta.Title,
		ArtistID:     "system",
		ArtistName:   meta.ArtistName,
		Duration:     meta.Duration,
		ThumbnailUrl: meta.ThumbnailURL,
		YoutubeID:    youtubeID,
		StorageUrl:   sql.NullString{Valid: false},
		Genre:        sql.NullString{String: "phonk", Valid: true},
		Source:       "user_favorited",
		IsApproved:   false,
		YtViewCount:  sql.NullInt64{Int64: 0, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to insert track: %w", err)
	}

	if u.publisher != nil {
		if err := u.publisher.PublishTrackDiscovered(ctx, trackID, youtubeID); err != nil {
			// Non-fatal: the track stays fully playable via the yt-dlp streaming
			// fallback in GetAudioStream even if the background download never fires.
			fmt.Printf("⚠️ failed to publish track.discovered for %s: %v\n", trackID, err)
		}
	}

	return nil
}

type ytVideoMeta struct {
	Title        string
	ArtistName   string
	ThumbnailURL string
	Duration     string
}

// fetchVideoMetadataByID calls YouTube's videos.list (≈1 quota unit) to fetch
// metadata for a single known video ID — far cheaper than another search.list
// call (100 units), used because at this point we already know the exact ID.
func (u *trackUsecase) fetchVideoMetadataByID(ctx context.Context, youtubeID string) (*ytVideoMeta, error) {
	url := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/videos?part=snippet,contentDetails&id=%s&key=%s",
		youtubeID, u.ytAPIKey,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ytResp struct {
		Items []struct {
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ytResp); err != nil {
		return nil, err
	}
	if len(ytResp.Items) == 0 {
		return nil, fmt.Errorf("video %s not found", youtubeID)
	}

	item := ytResp.Items[0]
	return &ytVideoMeta{
		Title:        item.Snippet.Title,
		ArtistName:   item.Snippet.ChannelTitle,
		ThumbnailURL: item.Snippet.Thumbnails.High.URL,
		Duration:     parseISO8601Duration(item.ContentDetails.Duration),
	}, nil
}

// parseISO8601Duration converts YouTube's "PT#M#S"-style duration into "MM:SS".
// Falls back to "00:00" for anything it can't parse (e.g. hour-long lives).
func parseISO8601Duration(iso string) string {
	iso = strings.TrimPrefix(iso, "PT")
	var hours, minutes, seconds int
	var num strings.Builder
	for _, r := range iso {
		switch {
		case r >= '0' && r <= '9':
			num.WriteRune(r)
		case r == 'H':
			hours, _ = strconv.Atoi(num.String())
			num.Reset()
		case r == 'M':
			minutes, _ = strconv.Atoi(num.String())
			num.Reset()
		case r == 'S':
			seconds, _ = strconv.Atoi(num.String())
			num.Reset()
		}
	}
	totalMinutes := hours*60 + minutes
	return fmt.Sprintf("%02d:%02d", totalMinutes, seconds)
}

func (u *trackUsecase) GetTrending(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error) {
	cacheKey := fmt.Sprintf("catalog:trending:%d", limit)

	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedTracks []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cachedTracks) == nil {
			return cachedTracks, nil
		}
	}

	// Real DB query — approved tracks ordered by play count
	dbTracks, err := u.repo.GetTrendingTracks(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("trending query failed: %w", err)
	}

	var grpcTracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		grpcTracks = append(grpcTracks, trackToProto(t))
	}

	if jsonBytes, err := json.Marshal(grpcTracks); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 10*time.Minute).Err()
	}

	return grpcTracks, nil
}

// ==========================================
// 🔄 2. PLAYBACK HISTORY TELEMETRY LOGIC
// ==========================================

func (u *trackUsecase) SyncTelemetry(ctx context.Context, userID string, trackID string, pos int32, completed bool) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user uuid format: %w", err)
	}

	err = u.repo.SyncPlaybackTelemetry(ctx, db.SyncPlaybackTelemetryParams{
		UserID:          userUUID,
		TrackID:         trackID,
		LastPositionSec: pos,
		IsCompleted:     completed,
	})
	if err != nil {
		return err
	}

	// Instantly invalidate the user's recent history cache item so their shelf reflects the updates immediately
	historyCacheKey := fmt.Sprintf("user:history:%s", userID)
	_ = u.rdb.Del(ctx, historyCacheKey).Err()

	if completed {
		err = u.repo.UpdateTrackStats(ctx, db.UpdateTrackStatsParams{
			PlayCountChange:  1,
			LikesCountChange: 0,
			TrackID:          trackID,
		})
		// Invalidate trending cache shelves since play counter data shifted metrics
		u.invalidateCachePattern(ctx, "catalog:trending:*")
	}

	return err
}

func (u *trackUsecase) GetRecentHistory(ctx context.Context, userID string, limit int32) ([]*trackpb.TrackMetadata, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid format: %w", err)
	}

	cacheKey := fmt.Sprintf("user:history:%s", userID)

	// ⚡ REDIS BYPASS LOOKUP
	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedTracks []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cachedTracks) == nil {
			return cachedTracks, nil
		}
	}

	historyRows, err := u.repo.GetRecentlyPlayed(ctx, db.GetRecentlyPlayedParams{
		UserID: userUUID,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}

	var grpcTracks []*trackpb.TrackMetadata
	for _, row := range historyRows {
		grpcTracks = append(grpcTracks, &trackpb.TrackMetadata{
			TrackId:           row.ID,
			Title:             row.Title,
			ArtistName:        row.ArtistName,
			Duration:          row.Duration,
			ThumbnailUrl:      row.ThumbnailUrl,
			OriginalYoutubeId: row.YoutubeID,
			PlayCount:         row.PlayCount,
			LikesCount:        row.LikesCount,
		})
	}

	// Cache personalized playback history for 5 minutes
	if jsonBytes, err := json.Marshal(grpcTracks); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 5*time.Minute).Err()
	}

	return grpcTracks, nil
}

// ==========================================
// 📁 3. ENGAGEMENT LAYER & USER OPERATIONS
// ==========================================

func (u *trackUsecase) SetInteraction(ctx context.Context, userID string, trackID string, isLiked bool) error {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user uuid format: %w", err)
	}

	if err := u.ensureTrackPersisted(ctx, trackID); err != nil {
		return fmt.Errorf("failed to persist track before interaction: %w", err)
	}

	err = u.repo.SetTrackInteraction(ctx, db.SetTrackInteractionParams{
		UserID:  userUUID,
		TrackID: trackID,
		IsLiked: isLiked,
	})
	if err != nil {
		return err
	}

	var likeDelta int32 = -1
	if isLiked {
		likeDelta = 1
	}

	// Clear global trending caches since engagement counters shifted position balances
	u.invalidateCachePattern(ctx, "catalog:trending:*")
	u.invalidateCachePattern(ctx, fmt.Sprintf("user:liked:%s:*", userID))

	return u.repo.UpdateTrackStats(ctx, db.UpdateTrackStatsParams{
		PlayCountChange:  0,
		LikesCountChange: likeDelta,
		TrackID:          trackID,
	})
}

func (u *trackUsecase) NewPlaylist(ctx context.Context, userID string, name string, coverURL string) (*trackpb.PlaylistResponse, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid: %w", err)
	}

	playlist, err := u.repo.CreatePlaylist(ctx, db.CreatePlaylistParams{
		UserID:        userUUID,
		Name:          name,
		CoverImageUrl: sql.NullString{String: coverURL, Valid: coverURL != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create playlist: %w", err)
	}

	return &trackpb.PlaylistResponse{
		PlaylistId:    playlist.ID.String(),
		Name:          playlist.Name,
		UserId:        playlist.UserID.String(),
		CoverImageUrl: playlist.CoverImageUrl.String,
	}, nil
}

func (u *trackUsecase) AddTrackToPlaylist(ctx context.Context, playlistID string, trackID string, requesterID string) error {
	playlistUUID, err := uuid.Parse(playlistID)
	if err != nil {
		return fmt.Errorf("invalid playlist uuid: %w", err)
	}

	playlist, err := u.repo.GetPlaylistByID(ctx, playlistUUID)
	if err != nil {
		return fmt.Errorf("playlist not found: %w", err)
	}
	if playlist.UserID.String() != requesterID {
		return ErrPlaylistAccessDenied
	}

	if err := u.ensureTrackPersisted(ctx, trackID); err != nil {
		return fmt.Errorf("failed to persist track before playlist add: %w", err)
	}

	return u.repo.AddTrackToPlaylist(ctx, db.AddTrackToPlaylistParams{
		PlaylistID: playlistUUID,
		TrackID:    trackID,
	})
}

func (u *trackUsecase) RemoveTrackFromPlaylist(ctx context.Context, playlistID string, trackID string, requesterID string) error {
	playlistUUID, err := uuid.Parse(playlistID)
	if err != nil {
		return fmt.Errorf("invalid playlist uuid: %w", err)
	}

	playlist, err := u.repo.GetPlaylistByID(ctx, playlistUUID)
	if err != nil {
		return fmt.Errorf("playlist not found: %w", err)
	}
	if playlist.UserID.String() != requesterID {
		return ErrPlaylistAccessDenied
	}

	return u.repo.RemoveTrackFromPlaylist(ctx, db.RemoveTrackFromPlaylistParams{
		PlaylistID: playlistUUID,
		TrackID:    trackID,
	})
}

// DeletePlaylist removes the playlist row itself; playlist_tracks has an
// ON DELETE CASCADE against playlists(id), so its join rows go with it, but
// the underlying shared tracks catalog is never touched — other users'
// playlists/favorites referencing the same tracks are unaffected.
func (u *trackUsecase) DeletePlaylist(ctx context.Context, playlistID string, requesterID string) error {
	playlistUUID, err := uuid.Parse(playlistID)
	if err != nil {
		return fmt.Errorf("invalid playlist uuid: %w", err)
	}

	playlist, err := u.repo.GetPlaylistByID(ctx, playlistUUID)
	if err != nil {
		return fmt.Errorf("playlist not found: %w", err)
	}
	if playlist.UserID.String() != requesterID {
		return ErrPlaylistAccessDenied
	}

	return u.repo.DeletePlaylist(ctx, playlistUUID)
}

func (u *trackUsecase) GetPlaylist(ctx context.Context, playlistID string, requesterID string) (*trackpb.GetPlaylistResponse, error) {
	playlistUUID, err := uuid.Parse(playlistID)
	if err != nil {
		return nil, fmt.Errorf("invalid playlist uuid: %w", err)
	}

	playlist, err := u.repo.GetPlaylistByID(ctx, playlistUUID)
	if err != nil {
		return nil, fmt.Errorf("playlist not found: %w", err)
	}
	if playlist.IsPrivate && playlist.UserID.String() != requesterID {
		return nil, ErrPlaylistAccessDenied
	}

	dbTracks, err := u.repo.GetPlaylistTracks(ctx, playlistUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playlist tracks: %w", err)
	}

	var tracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		tracks = append(tracks, trackToProto(t))
	}

	return &trackpb.GetPlaylistResponse{
		PlaylistId:    playlist.ID.String(),
		Name:          playlist.Name,
		UserId:        playlist.UserID.String(),
		CoverImageUrl: playlist.CoverImageUrl.String,
		IsPrivate:     playlist.IsPrivate,
		Tracks:        tracks,
	}, nil
}

func (u *trackUsecase) GetUserPlaylists(ctx context.Context, userID string) ([]*trackpb.PlaylistSummary, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid: %w", err)
	}

	rows, err := u.repo.GetUserPlaylists(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user playlists: %w", err)
	}

	var playlists []*trackpb.PlaylistSummary
	for _, row := range rows {
		playlists = append(playlists, &trackpb.PlaylistSummary{
			PlaylistId:    row.ID.String(),
			Name:          row.Name,
			CoverImageUrl: row.CoverImageUrl.String,
			IsPrivate:     row.IsPrivate,
			TrackCount:    int32(row.TrackCount),
		})
	}

	return playlists, nil
}

func (u *trackUsecase) GetForYou(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error) {
	cacheKey := fmt.Sprintf("foryou:%d", limit)
	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cached []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cached) == nil {
			return cached, nil
		}
	}

	dbTracks, err := u.repo.GetForYouTracks(ctx, limit)
	if err != nil {
		return nil, err
	}

	var tracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		tracks = append(tracks, trackToProto(t))
	}

	if jsonBytes, err := json.Marshal(tracks); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 3*time.Minute).Err()
	}

	return tracks, nil
}

func (u *trackUsecase) SeedTrack(ctx context.Context, req *trackpb.SeedTrackRequest) (string, error) {
	// Check duplicate
	existing, _ := u.repo.GetTrackByYoutubeID(ctx, req.GetYoutubeId())
	if existing.YoutubeID == req.GetYoutubeId() {
		return existing.ID, nil
	}

	id := uuid.New().String()
	_, err := u.repo.InsertTrack(ctx, db.InsertTrackParams{
		ID:           id,
		Title:        req.GetTitle(),
		ArtistID:     "system",
		ArtistName:   req.GetArtistName(),
		Duration:     "00:00",
		ThumbnailUrl: req.GetThumbnailUrl(),
		YoutubeID:    req.GetYoutubeId(),
		StorageUrl:   sql.NullString{String: req.GetStorageUrl(), Valid: req.GetStorageUrl() != ""},
		Genre:        sql.NullString{String: req.GetGenre(), Valid: req.GetGenre() != ""},
		Source:       "manual",
		IsApproved:   true,
		YtViewCount:  sql.NullInt64{Int64: 0, Valid: true},
	})
	if err != nil {
		return "", err
	}

	// Invalidate trending cache
	_ = u.rdb.Del(ctx, "catalog:trending:*").Err()
	return id, nil
}

func (u *trackUsecase) ListTracksAdmin(ctx context.Context, page, limit int32) ([]*trackpb.TrackMetadata, int32, error) {
	if limit <= 0 {
		limit = 20
	}
	dbTracks, err := u.repo.GetAllTracksAdmin(ctx, db.GetAllTracksAdminParams{
		Limit:  limit,
		Offset: page * limit,
	})
	if err != nil {
		return nil, 0, err
	}

	var tracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		tracks = append(tracks, trackToProto(t))
	}

	return tracks, int32(len(tracks)), nil
}

func (u *trackUsecase) ApproveTrack(ctx context.Context, trackID string) error {
	_ = u.rdb.Del(ctx, "catalog:trending:*").Err()
	return u.repo.ApproveTrack(ctx, trackID)
}

func (u *trackUsecase) RejectTrack(ctx context.Context, trackID string) error {
	return u.repo.RejectTrack(ctx, trackID)
}

func (u *trackUsecase) FeatureTrack(ctx context.Context, trackID string, featured bool) error {
	return u.repo.ToggleFeatureTrack(ctx, db.ToggleFeatureTrackParams{
		ID:         trackID,
		IsFeatured: featured,
	})
}

func (u *trackUsecase) DeleteTrack(ctx context.Context, trackID string) error {
	_ = u.rdb.Del(ctx, fmt.Sprintf("stream:%s", trackID)).Err()
	return u.repo.DeleteTrack(ctx, trackID)
}

func (u *trackUsecase) GetAdminStats(ctx context.Context) (*trackpb.AdminStatsResponse, error) {
	trending, err := u.repo.GetTrendingTracks(ctx, 1000)
	if err != nil {
		return nil, err
	}

	var totalPlays int32
	var pendingCount int32
	for _, t := range trending {
		totalPlays += t.PlayCount
		if !t.IsApproved && !t.IsRejected {
			pendingCount++
		}
	}

	return &trackpb.AdminStatsResponse{
		TotalTracks:   int32(len(trending)),
		TotalPlays:    totalPlays,
		PendingTracks: pendingCount,
	}, nil
}

func (u *trackUsecase) GetLikedTracks(ctx context.Context, userID string, limit int32, page int32) ([]*trackpb.TrackMetadata, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid: %w", err)
	}

	cacheKey := fmt.Sprintf("user:liked:%s:p%d", userID, page)
	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cached []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cached) == nil {
			return cached, nil
		}
	}

	if limit <= 0 {
		limit = 20
	}

	dbTracks, err := u.repo.GetLikedTracks(ctx, db.GetLikedTracksParams{
		UserID:  userUUID,
		Limit:   limit,
		Column3: page,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch liked tracks: %w", err)
	}

	var tracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		tracks = append(tracks, trackToProto(t))
	}

	if jsonBytes, err := json.Marshal(tracks); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 5*time.Minute).Err()
	}

	return tracks, nil
}

// invalidateCachePattern deletes all Redis keys matching a glob pattern.
// Redis DEL only accepts exact key names, so matching keys must be discovered via SCAN first.
func (u *trackUsecase) invalidateCachePattern(ctx context.Context, pattern string) {
	var keys []string
	iter := u.rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		_ = u.rdb.Del(ctx, keys...).Err()
	}
}

// ─── SHARED HELPER ───────────────────────────────────────────────────────────

func trackToProto(t db.Track) *trackpb.TrackMetadata {
	return &trackpb.TrackMetadata{
		TrackId:           t.ID,
		Title:             t.Title,
		ArtistName:        t.ArtistName,
		Duration:          t.Duration,
		ThumbnailUrl:      t.ThumbnailUrl,
		OriginalYoutubeId: t.YoutubeID,
		PlayCount:         t.PlayCount,
		LikesCount:        t.LikesCount,
		StorageUrl:        t.StorageUrl.String,
		Genre:             t.Genre.String,
		IsFeatured:        t.IsFeatured,
		IsApproved:        t.IsApproved,
		Source:            t.Source,
	}
}
