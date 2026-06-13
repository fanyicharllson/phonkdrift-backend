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
	return s.trackProxy.client.SyncPlaybackTelemetry(ctx, req)
}

func (s *GatewayServer) GetRecentlyPlayed(ctx context.Context, req *trackpb.RecentlyPlayedRequest) (*trackpb.RecentlyPlayedResponse, error) {
	return s.trackProxy.client.GetRecentlyPlayed(ctx, req)
}

func (s *GatewayServer) SetTrackInteraction(ctx context.Context, req *trackpb.InteractionRequest) (*trackpb.InteractionResponse, error) {
	return s.trackProxy.client.SetTrackInteraction(ctx, req)
}

func (s *GatewayServer) CreatePlaylist(ctx context.Context, req *trackpb.CreatePlaylistRequest) (*trackpb.PlaylistResponse, error) {
	return s.trackProxy.client.CreatePlaylist(ctx, req)
}

func (s *GatewayServer) AddToPlaylist(ctx context.Context, req *trackpb.PlaylistTrackRequest) (*trackpb.PlaylistActionResponse, error) {
	return s.trackProxy.client.AddToPlaylist(ctx, req)
}