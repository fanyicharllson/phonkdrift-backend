package grpc

import (
	"context"
	"errors"
	"log"

	"github.com/fanyicharllson/phonkdrift-backend/internal/track-service/usecase"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TrackGRPCHandler struct {
	trackpb.UnimplementedTrackServiceServer
	usecase usecase.TrackUsecase
}

func NewTrackGRPCHandler(uc usecase.TrackUsecase) *TrackGRPCHandler {
	return &TrackGRPCHandler{
		usecase: uc,
	}
}

// ==========================================
// 🔍 1. TRACK DISCOVERY LAYER
// ==========================================

func (h *TrackGRPCHandler) GetStreamURL(ctx context.Context, req *trackpb.StreamRequest) (*trackpb.StreamResponse, error) {
	log.Printf("📥 Received GetStreamURL request for Youtube ID: %s", req.GetOriginalYoutubeId())

	if req.GetOriginalYoutubeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "target youtube_id must be provided")
	}

	log.Println("⏳ Calling yt-dlp extractor usecase...")
	url, expiresAt, err := h.usecase.GetAudioStream(ctx, req.GetOriginalYoutubeId())
	if err != nil {
		log.Printf("❌ Usecase extraction error: %v", err)
		return nil, status.Errorf(codes.Internal, "audio streaming engine failure: %v", err)
	}

	log.Printf("✅ Stream successfully extracted: %s...", url[:30])
	return &trackpb.StreamResponse{
		StreamUrl:     url,
		LinkExpiresAt: expiresAt,
	}, nil
}

func (h *TrackGRPCHandler) SearchTrack(ctx context.Context, req *trackpb.SearchRequest) (*trackpb.SearchResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "search query cannot be empty")
	}

	// Forward to usecase (which will query Postgres or search YouTube via yt-dlp if not indexed)
	tracks, err := h.usecase.SearchTracks(ctx, req.GetQuery(), req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to execute track search: %v", err)
	}

	return &trackpb.SearchResponse{Tracks: tracks}, nil
}

func (h *TrackGRPCHandler) GetTrendingTracks(ctx context.Context, req *trackpb.TrendingRequest) (*trackpb.TrendingResponse, error) {
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 10 // Default fallback shelf size
	}

	tracks, err := h.usecase.GetTrending(ctx, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch trending catalog: %v", err)
	}

	return &trackpb.TrendingResponse{Tracks: tracks}, nil
}

// ==========================================
// 🔄 2. TELEMETRY & PLAYBACK SYNC LAYER
// ==========================================

func (h *TrackGRPCHandler) SyncPlaybackTelemetry(ctx context.Context, req *trackpb.PlaybackTelemetryRequest) (*trackpb.PlaybackTelemetryResponse, error) {
	err := h.usecase.SyncTelemetry(
		ctx,
		req.GetUserId(),
		req.GetTrackId(),
		req.GetLastPositionSeconds(),
		req.GetIsCompleted(),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to commit state history update: %v", err)
	}

	return &trackpb.PlaybackTelemetryResponse{Success: true}, nil
}

func (h *TrackGRPCHandler) GetRecentlyPlayed(ctx context.Context, req *trackpb.RecentlyPlayedRequest) (*trackpb.RecentlyPlayedResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id context missing")
	}

	tracks, err := h.usecase.GetRecentHistory(ctx, req.GetUserId(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve playback history: %v", err)
	}

	return &trackpb.RecentlyPlayedResponse{Tracks: tracks}, nil
}

// ==========================================
// 📁 3. USER LIBRARY & PLAYLIST LAYER
// ==========================================

func (h *TrackGRPCHandler) SetTrackInteraction(ctx context.Context, req *trackpb.InteractionRequest) (*trackpb.InteractionResponse, error) {
	err := h.usecase.SetInteraction(ctx, req.GetUserId(), req.GetTrackId(), req.GetIsLiked())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to register interaction update: %v", err)
	}

	return &trackpb.InteractionResponse{Success: true}, nil
}

func (h *TrackGRPCHandler) CreatePlaylist(ctx context.Context, req *trackpb.CreatePlaylistRequest) (*trackpb.PlaylistResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "playlist name cannot be blank")
	}

	playlist, err := h.usecase.NewPlaylist(ctx, req.GetUserId(), req.GetName(), req.GetCoverImageUrl())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create playlist record: %v", err)
	}

	return playlist, nil
}

func (h *TrackGRPCHandler) AddToPlaylist(ctx context.Context, req *trackpb.PlaylistTrackRequest) (*trackpb.PlaylistActionResponse, error) {
	err := h.usecase.AddTrackToPlaylist(ctx, req.GetPlaylistId(), req.GetTrackId(), req.GetUserId())
	if err != nil {
		if errors.Is(err, usecase.ErrPlaylistAccessDenied) {
			return nil, status.Error(codes.PermissionDenied, "you do not own this playlist")
		}
		return nil, status.Errorf(codes.Internal, "failed to append track to collection: %v", err)
	}

	return &trackpb.PlaylistActionResponse{
		Success: true,
		Message: "Track successfully appended to playlist",
	}, nil
}

func (h *TrackGRPCHandler) DeletePlaylist(ctx context.Context, req *trackpb.DeletePlaylistRequest) (*trackpb.PlaylistActionResponse, error) {
	if req.GetPlaylistId() == "" {
		return nil, status.Error(codes.InvalidArgument, "playlist_id is required")
	}

	err := h.usecase.DeletePlaylist(ctx, req.GetPlaylistId(), req.GetUserId())
	if err != nil {
		if errors.Is(err, usecase.ErrPlaylistAccessDenied) {
			return nil, status.Error(codes.PermissionDenied, "you do not own this playlist")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete playlist: %v", err)
	}

	return &trackpb.PlaylistActionResponse{
		Success: true,
		Message: "Playlist deleted",
	}, nil
}

func (h *TrackGRPCHandler) GetPlaylist(ctx context.Context, req *trackpb.GetPlaylistRequest) (*trackpb.GetPlaylistResponse, error) {
	if req.GetPlaylistId() == "" {
		return nil, status.Error(codes.InvalidArgument, "playlist_id is required")
	}

	res, err := h.usecase.GetPlaylist(ctx, req.GetPlaylistId(), req.GetUserId())
	if err != nil {
		if errors.Is(err, usecase.ErrPlaylistAccessDenied) {
			return nil, status.Error(codes.PermissionDenied, "this playlist is private")
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch playlist: %v", err)
	}
	return res, nil
}

func (h *TrackGRPCHandler) GetUserPlaylists(ctx context.Context, req *trackpb.GetUserPlaylistsRequest) (*trackpb.GetUserPlaylistsResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	playlists, err := h.usecase.GetUserPlaylists(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch user playlists: %v", err)
	}
	return &trackpb.GetUserPlaylistsResponse{Playlists: playlists}, nil
}

// ==========================================
// 🔨 4. ADMIN OPERATIONS
// ==========================================

func (h *TrackGRPCHandler) GetForYou(ctx context.Context, req *trackpb.ForYouRequest) (*trackpb.ForYouResponse, error) {
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 20
	}
	tracks, err := h.usecase.GetForYou(ctx, limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch ForYou feed: %v", err)
	}
	return &trackpb.ForYouResponse{Tracks: tracks}, nil
}

func (h *TrackGRPCHandler) SeedTrack(ctx context.Context, req *trackpb.SeedTrackRequest) (*trackpb.SeedTrackResponse, error) {
	trackID, err := h.usecase.SeedTrack(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "seed failed: %v", err)
	}
	return &trackpb.SeedTrackResponse{
		TrackId:   trackID,
		StorageUrl: req.GetStorageUrl(),
	}, nil
}

func (h *TrackGRPCHandler) ListTracksAdmin(ctx context.Context, req *trackpb.ListTracksAdminRequest) (*trackpb.ListTracksAdminResponse, error) {
	tracks, total, err := h.usecase.ListTracksAdmin(ctx, req.GetPage(), req.GetLimit())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list tracks: %v", err)
	}
	return &trackpb.ListTracksAdminResponse{Tracks: tracks, Total: total}, nil
}

func (h *TrackGRPCHandler) ApproveTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	err := h.usecase.ApproveTrack(ctx, req.GetTrackId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "approve failed: %v", err)
	}
	return &trackpb.TrackActionResponse{Success: true, Message: "Track approved"}, nil
}

func (h *TrackGRPCHandler) RejectTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	err := h.usecase.RejectTrack(ctx, req.GetTrackId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reject failed: %v", err)
	}
	return &trackpb.TrackActionResponse{Success: true, Message: "Track rejected"}, nil
}

func (h *TrackGRPCHandler) FeatureTrack(ctx context.Context, req *trackpb.FeatureTrackRequest) (*trackpb.TrackActionResponse, error) {
	err := h.usecase.FeatureTrack(ctx, req.GetTrackId(), req.GetIsFeatured())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "feature toggle failed: %v", err)
	}
	return &trackpb.TrackActionResponse{Success: true, Message: "Feature status updated"}, nil
}

func (h *TrackGRPCHandler) DeleteTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	err := h.usecase.DeleteTrack(ctx, req.GetTrackId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
	}
	return &trackpb.TrackActionResponse{Success: true, Message: "Track deleted"}, nil
}

func (h *TrackGRPCHandler) GetAdminStats(ctx context.Context, _ *trackpb.Empty) (*trackpb.AdminStatsResponse, error) {
	stats, err := h.usecase.GetAdminStats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stats fetch failed: %v", err)
	}
	return stats, nil
}

func (h *TrackGRPCHandler) GetLikedTracks(ctx context.Context, req *trackpb.GetLikedTracksRequest) (*trackpb.GetLikedTracksResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 20
	}

	tracks, err := h.usecase.GetLikedTracks(ctx, req.GetUserId(), limit, req.GetPage())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch liked tracks: %v", err)
	}
	return &trackpb.GetLikedTracksResponse{
		Tracks: tracks,
		Total:  int32(len(tracks)),
	}, nil
}