package server

import (
	"context"

	chatpb "github.com/fanyicharllson/phonkdrift-backend/pb/chat"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ChatProxy holds the gRPC client connection to the internal chat microservice
type ChatProxy struct {
	client chatpb.ChatServiceClient
}

func NewChatProxy(targetAddr string) (*ChatProxy, error) {
	conn, err := grpc.NewClient(targetAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	return &ChatProxy{
		client: chatpb.NewChatServiceClient(conn),
	}, nil
}

func (s *GatewayServer) JoinCommunity(ctx context.Context, req *chatpb.JoinCommunityRequest) (*chatpb.JoinCommunityResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.JoinCommunity(ctx, req)
}

func (s *GatewayServer) LeaveCommunity(ctx context.Context, req *chatpb.LeaveCommunityRequest) (*chatpb.LeaveCommunityResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.LeaveCommunity(ctx, req)
}

func (s *GatewayServer) IsCommunityMember(ctx context.Context, req *chatpb.IsCommunityMemberRequest) (*chatpb.IsCommunityMemberResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.IsCommunityMember(ctx, req)
}

func (s *GatewayServer) SendMessage(ctx context.Context, req *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.SendMessage(ctx, req)
}

func (s *GatewayServer) GetMessages(ctx context.Context, req *chatpb.GetMessagesRequest) (*chatpb.GetMessagesResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.GetMessages(ctx, req)
}

func (s *GatewayServer) GetCommunityStats(ctx context.Context, req *chatpb.GetCommunityStatsRequest) (*chatpb.GetCommunityStatsResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.GetCommunityStats(ctx, req)
}

func (s *GatewayServer) GetCommunityMembers(ctx context.Context, req *chatpb.GetCommunityMembersRequest) (*chatpb.GetCommunityMembersResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.chatProxy.client.GetCommunityMembers(ctx, req)
}

// SubscribeToChat relays the streaming RPC through to chat-service: opens an
// outgoing stream downstream, then forwards every message it receives back
// to the mobile client's incoming stream.
func (s *GatewayServer) SubscribeToChat(req *chatpb.SubscribeRequest, stream chatpb.ChatService_SubscribeToChatServer) error {
	req.UserId = verifiedUserID(stream.Context())

	downstream, err := s.chatProxy.client.SubscribeToChat(stream.Context(), req)
	if err != nil {
		return err
	}

	for {
		msg, err := downstream.Recv()
		if err != nil {
			return err
		}
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
}
