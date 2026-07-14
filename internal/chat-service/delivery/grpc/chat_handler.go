package grpc

import (
	"context"
	"errors"

	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/broadcaster"
	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/domain"
	chatpb "github.com/fanyicharllson/phonkdrift-backend/pb/chat"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ChatGRPCHandler struct {
	chatpb.UnimplementedChatServiceServer
	usecase     domain.ChatUsecase
	broadcaster *broadcaster.Broadcaster
}

func NewChatGRPCHandler(uc domain.ChatUsecase, bc *broadcaster.Broadcaster) *ChatGRPCHandler {
	return &ChatGRPCHandler{usecase: uc, broadcaster: bc}
}

func (h *ChatGRPCHandler) JoinCommunity(ctx context.Context, req *chatpb.JoinCommunityRequest) (*chatpb.JoinCommunityResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if err := h.usecase.JoinCommunity(ctx, req.GetUserId()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to join community: %v", err)
	}
	return &chatpb.JoinCommunityResponse{Success: true}, nil
}

func (h *ChatGRPCHandler) LeaveCommunity(ctx context.Context, req *chatpb.LeaveCommunityRequest) (*chatpb.LeaveCommunityResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if err := h.usecase.LeaveCommunity(ctx, req.GetUserId()); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to leave community: %v", err)
	}
	return &chatpb.LeaveCommunityResponse{Success: true}, nil
}

func (h *ChatGRPCHandler) IsCommunityMember(ctx context.Context, req *chatpb.IsCommunityMemberRequest) (*chatpb.IsCommunityMemberResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	isMember, err := h.usecase.IsCommunityMember(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check membership: %v", err)
	}
	return &chatpb.IsCommunityMemberResponse{IsMember: isMember}, nil
}

func (h *ChatGRPCHandler) SendMessage(ctx context.Context, req *chatpb.SendMessageRequest) (*chatpb.SendMessageResponse, error) {
	msg, err := h.usecase.SendMessage(ctx, req.GetUserId(), req.GetContent(), req.GetMediaUrl(), req.GetMessageType(), req.GetReplyToId())
	if err != nil {
		if errors.Is(err, domain.ErrNotCommunityMember) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &chatpb.SendMessageResponse{
		Success: true,
		Message: msgToProto(msg),
	}, nil
}

func (h *ChatGRPCHandler) GetMessages(ctx context.Context, req *chatpb.GetMessagesRequest) (*chatpb.GetMessagesResponse, error) {
	messages, err := h.usecase.GetMessages(ctx, req.GetUserId(), req.GetBeforeTimestamp(), req.GetLimit())
	if err != nil {
		if errors.Is(err, domain.ErrNotCommunityMember) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch messages: %v", err)
	}

	protoMessages := make([]*chatpb.ChatMessage, 0, len(messages))
	for _, m := range messages {
		protoMessages = append(protoMessages, msgToProto(m))
	}
	return &chatpb.GetMessagesResponse{Messages: protoMessages}, nil
}

func (h *ChatGRPCHandler) SubscribeToChat(req *chatpb.SubscribeRequest, stream chatpb.ChatService_SubscribeToChatServer) error {
	isMember, err := h.usecase.IsCommunityMember(stream.Context(), req.GetUserId())
	if err != nil {
		return status.Errorf(codes.Internal, "failed to verify membership: %v", err)
	}
	if !isMember {
		return status.Error(codes.PermissionDenied, domain.ErrNotCommunityMember.Error())
	}

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(msgToProto(msg)); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (h *ChatGRPCHandler) GetCommunityStats(ctx context.Context, req *chatpb.GetCommunityStatsRequest) (*chatpb.GetCommunityStatsResponse, error) {
	total, err := h.usecase.GetCommunityStats(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, domain.ErrNotCommunityMember) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to fetch community stats: %v", err)
	}
	return &chatpb.GetCommunityStatsResponse{TotalMembers: int32(total)}, nil
}

func (h *ChatGRPCHandler) GetCommunityMembers(ctx context.Context, req *chatpb.GetCommunityMembersRequest) (*chatpb.GetCommunityMembersResponse, error) {
	members, total, err := h.usecase.GetCommunityMembers(ctx, req.GetUserId(), req.GetPage(), req.GetLimit())
	if err != nil {
		if errors.Is(err, domain.ErrNotCommunityMember) {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to list community members: %v", err)
	}

	protoMembers := make([]*chatpb.CommunityMember, 0, len(members))
	for _, m := range members {
		protoMembers = append(protoMembers, &chatpb.CommunityMember{
			UserId:    m.UserID,
			Username:  m.Username,
			AvatarUrl: m.AvatarURL,
			JoinedAt:  m.JoinedAt.Unix(),
			Badge:     m.Badge,
		})
	}
	return &chatpb.GetCommunityMembersResponse{
		Members: protoMembers,
		Total:   int32(total),
	}, nil
}

func msgToProto(m *domain.ChatMessage) *chatpb.ChatMessage {
	return &chatpb.ChatMessage{
		Id:                    m.ID,
		UserId:                m.UserID,
		Username:              m.Username,
		AvatarUrl:             m.AvatarURL,
		Content:               m.Content,
		MediaUrl:              m.MediaURL,
		MessageType:           m.MessageType,
		ReplyToId:             m.ReplyToID,
		ReplyToUserId:         m.ReplyToUserID,
		ReplyToUsername:       m.ReplyToUsername,
		ReplyToContentSnippet: m.ReplyToContentSnippet,
		CreatedAt:             m.CreatedAt.Unix(),
	}
}
