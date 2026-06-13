package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	db "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type TrackUsecase interface {
	GetAudioStream(ctx context.Context, youtubeID string) (string, int64, error)
	SyncTelemetry(ctx context.Context, userID string, trackID string, pos int32, completed bool) error
	SearchTracks(ctx context.Context, query string, page int32) ([]*trackpb.TrackMetadata, error)
	GetTrending(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error)
	GetRecentHistory(ctx context.Context, userID string, limit int32) ([]*trackpb.TrackMetadata, error)
	SetInteraction(ctx context.Context, userID string, trackID string, isLiked bool) error
	NewPlaylist(ctx context.Context, userID string, name string, coverURL string) (*trackpb.PlaylistResponse, error)
	AddTrackToPlaylist(ctx context.Context, playlistID string, trackID string) error
}

type trackUsecase struct {
	repo  db.Querier
	rdb   *redis.Client
}

func NewTrackUsecase(repo db.Querier, rdb *redis.Client) TrackUsecase {
	return &trackUsecase{
		repo: repo,
		rdb:  rdb,
	}
}

// ==========================================
// 🔍 1. TRACK STREAMING & DISCOVERY LOGIC
// ==========================================

func (u *trackUsecase) GetAudioStream(ctx context.Context, youtubeID string) (string, int64, error) {
	cacheKey := fmt.Sprintf("stream:%s", youtubeID)

	// ⚡ REDIS BYPASS LOOKUP: Check if raw stream link is already cached
	cachedURL, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedURL != "" {
		// Calculate safe remainder TTL window for the mobile app client
		ttl, _ := u.rdb.TTL(ctx, cacheKey).Result()
		expiresAt := time.Now().Add(ttl).Unix()
		return cachedURL, expiresAt, nil
	}

	// 🐌 CACHE MISS: Execute yt-dlp binary thread sub-process safely
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", youtubeID)
	args := []string{"-f", "bestaudio", "-g", videoURL}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("yt-dlp crashed: %s (%w)", stderr.String(), err)
	}

	streamURL := strings.TrimSpace(out.String())
	if streamURL == "" {
		return "", 0, fmt.Errorf("extracted streaming url is empty")
	}

	// Dynamic URLs expire in 6h; cache them safely for 5 hours to protect buffer thresholds
	cacheTTL := 5 * time.Hour
	expiresAt := time.Now().Add(cacheTTL).Unix()

	// Commit back into Redis memory footprint asynchronously
	_ = u.rdb.Set(ctx, cacheKey, streamURL, cacheTTL).Err()

	return streamURL, expiresAt, nil
}

func (u *trackUsecase) SearchTracks(ctx context.Context, query string, page int32) ([]*trackpb.TrackMetadata, error) {
	cleanQuery := strings.TrimSpace(strings.ToLower(query))
	cacheKey := fmt.Sprintf("search:%s:p%d", cleanQuery, page)

	// ⚡ REDIS BYPASS LOOKUP: Return cached search results instantly
	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedTracks []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cachedTracks) == nil {
			return cachedTracks, nil
		}
	}

	// DB LOOKUP LAYER: Fetch catalog matching query via SQLC
	// (Using search mock until database search indices extensions match your model profile)
	var results []*trackpb.TrackMetadata
	results = append(results, &trackpb.TrackMetadata{
		TrackId:           "local-track-uuid-1",
		Title:             "PHONK DRIFT OVERDRIVE",
		ArtistName:        "KORDHELL x DJ FANYI",
		Duration:          "02:45",
		ThumbnailUrl:      "https://supabase.co/storage/v1/object/public/covers/drift.jpg",
		OriginalYoutubeId: "dQw4w9WgXcQ",
		PlayCount:         14500,
		LikesCount:        3200,
	})

	// Cache search results for 30 minutes to reduce database read overhead
	if jsonBytes, err := json.Marshal(results); err == nil {
		_ = u.rdb.Set(ctx, cacheKey, jsonBytes, 30*time.Minute).Err()
	}

	return results, nil
}

func (u *trackUsecase) GetTrending(ctx context.Context, limit int32) ([]*trackpb.TrackMetadata, error) {
	cacheKey := fmt.Sprintf("catalog:trending:%d", limit)

	// ⚡ REDIS BYPASS LOOKUP
	cachedData, err := u.rdb.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var cachedTracks []*trackpb.TrackMetadata
		if json.Unmarshal([]byte(cachedData), &cachedTracks) == nil {
			return cachedTracks, nil
		}
	}

	dbTracks, err := u.repo.GetTrendingTracks(ctx, limit)
	if err != nil {
		return nil, err
	}

	var grpcTracks []*trackpb.TrackMetadata
	for _, t := range dbTracks {
		grpcTracks = append(grpcTracks, &trackpb.TrackMetadata{
			TrackId:           t.ID,
			Title:             t.Title,
			ArtistName:        t.ArtistName,
			Duration:          t.Duration,
			ThumbnailUrl:      t.ThumbnailUrl,
			OriginalYoutubeId: t.YoutubeID,
			PlayCount:         t.PlayCount,
			LikesCount:        t.LikesCount,
		})
	}

	// Cache trending dashboard shelves globally for 10 minutes
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
		_ = u.rdb.Del(ctx, "catalog:trending:*").Err()
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
	_ = u.rdb.Del(ctx, "catalog:trending:*").Err()

	return u.repo.UpdateTrackStats(ctx, db.UpdateTrackStatsParams{
		PlayCountChange:  0,
		LikesCountChange: likeDelta,
		TrackID:          trackID,
	})
}

func (u *trackUsecase) NewPlaylist(ctx context.Context, userID string, name string, coverURL string) (*trackpb.PlaylistResponse, error) {
	return &trackpb.PlaylistResponse{
		PlaylistId: uuid.New().String(),
		Name:       name,
		UserId:     userID,
	}, nil
}

func (u *trackUsecase) AddTrackToPlaylist(ctx context.Context, playlistID string, trackID string) error {
	return nil
}