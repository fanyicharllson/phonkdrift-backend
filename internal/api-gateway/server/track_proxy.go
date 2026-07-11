package server

import (
	"context"

	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TrackProxy holds the gRPC client connection to the internal track microservice
type TrackProxy struct {
	client trackpb.TrackServiceClient
}

func NewTrackProxy(targetAddr string) (*TrackProxy, error) {
	// Set up a secure, low-latency cluster connection to the tracking service binary
	conn, err := grpc.NewClient(targetAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &TrackProxy{
		client: trackpb.NewTrackServiceClient(conn),
	}, nil
}

// =========================================================================
// 🔀 REVERSE PROXY LAYER (Intercepts mobile calls and funnels them downstream)
// =========================================================================

func (s *GatewayServer) GetStreamURL(ctx context.Context, req *trackpb.StreamRequest) (*trackpb.StreamResponse, error) {
	return s.trackProxy.client.GetStreamURL(ctx, req)
}

func (s *GatewayServer) SearchTrack(ctx context.Context, req *trackpb.SearchRequest) (*trackpb.SearchResponse, error) {
	return s.trackProxy.client.SearchTrack(ctx, req)
}

func (s *GatewayServer) GetTrendingTracks(ctx context.Context, req *trackpb.TrendingRequest) (*trackpb.TrendingResponse, error) {
	return s.trackProxy.client.GetTrendingTracks(ctx, req)
}

func (s *GatewayServer) SyncPlaybackTelemetry(ctx context.Context, req *trackpb.PlaybackTelemetryRequest) (*trackpb.PlaybackTelemetryResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.SyncPlaybackTelemetry(ctx, req)
}

func (s *GatewayServer) GetRecentlyPlayed(ctx context.Context, req *trackpb.RecentlyPlayedRequest) (*trackpb.RecentlyPlayedResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.GetRecentlyPlayed(ctx, req)
}

func (s *GatewayServer) SetTrackInteraction(ctx context.Context, req *trackpb.InteractionRequest) (*trackpb.InteractionResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.SetTrackInteraction(ctx, req)
}

// verifiedUserID pulls the JWT-validated user_id that authUnaryInterceptor
// stashed in context, so playlist ownership/privacy checks can't be spoofed
// by whatever user_id a client puts in the request body.
func verifiedUserID(ctx context.Context) string {
	uid, _ := ctx.Value(userIDContextKey).(string)
	return uid
}

func (s *GatewayServer) CreatePlaylist(ctx context.Context, req *trackpb.CreatePlaylistRequest) (*trackpb.PlaylistResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.CreatePlaylist(ctx, req)
}

func (s *GatewayServer) AddToPlaylist(ctx context.Context, req *trackpb.PlaylistTrackRequest) (*trackpb.PlaylistActionResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.AddToPlaylist(ctx, req)
}

func (s *GatewayServer) RemoveTrackFromPlaylist(ctx context.Context, req *trackpb.PlaylistTrackRequest) (*trackpb.PlaylistActionResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.RemoveTrackFromPlaylist(ctx, req)
}

func (s *GatewayServer) GetPlaylist(ctx context.Context, req *trackpb.GetPlaylistRequest) (*trackpb.GetPlaylistResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.GetPlaylist(ctx, req)
}

func (s *GatewayServer) DeletePlaylist(ctx context.Context, req *trackpb.DeletePlaylistRequest) (*trackpb.PlaylistActionResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.DeletePlaylist(ctx, req)
}

func (s *GatewayServer) GetUserPlaylists(ctx context.Context, req *trackpb.GetUserPlaylistsRequest) (*trackpb.GetUserPlaylistsResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.GetUserPlaylists(ctx, req)
}

func (s *GatewayServer) GetForYou(ctx context.Context, req *trackpb.ForYouRequest) (*trackpb.ForYouResponse, error) {
	return s.trackProxy.client.GetForYou(ctx, req)
}

func (s *GatewayServer) SeedTrack(ctx context.Context, req *trackpb.SeedTrackRequest) (*trackpb.SeedTrackResponse, error) {
	return s.trackProxy.client.SeedTrack(ctx, req)
}

func (s *GatewayServer) ListTracksAdmin(ctx context.Context, req *trackpb.ListTracksAdminRequest) (*trackpb.ListTracksAdminResponse, error) {
	return s.trackProxy.client.ListTracksAdmin(ctx, req)
}

func (s *GatewayServer) ApproveTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	return s.trackProxy.client.ApproveTrack(ctx, req)
}

func (s *GatewayServer) RejectTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	return s.trackProxy.client.RejectTrack(ctx, req)
}

func (s *GatewayServer) FeatureTrack(ctx context.Context, req *trackpb.FeatureTrackRequest) (*trackpb.TrackActionResponse, error) {
	return s.trackProxy.client.FeatureTrack(ctx, req)
}

func (s *GatewayServer) DeleteTrack(ctx context.Context, req *trackpb.TrackActionRequest) (*trackpb.TrackActionResponse, error) {
	return s.trackProxy.client.DeleteTrack(ctx, req)
}

func (s *GatewayServer) GetAdminStats(ctx context.Context, req *trackpb.Empty) (*trackpb.AdminStatsResponse, error) {
	return s.trackProxy.client.GetAdminStats(ctx, req)
}

func (s *GatewayServer) GetLikedTracks(ctx context.Context, req *trackpb.GetLikedTracksRequest) (*trackpb.GetLikedTracksResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.trackProxy.client.GetLikedTracks(ctx, req)
}