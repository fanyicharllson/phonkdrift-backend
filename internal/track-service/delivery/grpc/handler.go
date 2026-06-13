package grpc

import (
	"context"
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
	err := h.usecase.AddTrackToPlaylist(ctx, req.GetPlaylistId(), req.GetTrackId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to append track to collection: %v", err)
	}

	return &trackpb.PlaylistActionResponse{
		Success: true,
		Message: "Track successfully appended to playlist",
	}, nil
}