package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/broadcaster"
	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/domain"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
)

const maxContentLength = 2000

type chatUsecase struct {
	repo        domain.ChatRepository
	broadcaster *broadcaster.Broadcaster
	authClient  authpb.AuthServiceClient
}

func NewChatUsecase(repo domain.ChatRepository, bc *broadcaster.Broadcaster, authClient authpb.AuthServiceClient) domain.ChatUsecase {
	return &chatUsecase{repo: repo, broadcaster: bc, authClient: authClient}
}

func (u *chatUsecase) JoinCommunity(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user_id is required")
	}

	userRes, err := u.authClient.GetUser(ctx, &authpb.GetUserRequest{UserId: userID})
	if err != nil {
		return fmt.Errorf("failed to look up profile: %w", err)
	}

	isNew, err := u.repo.JoinCommunity(ctx, userID, userRes.GetUser().GetUsername(), userRes.GetUser().GetAvatarUrl())
	if err != nil {
		return fmt.Errorf("failed to join community: %w", err)
	}

	if isNew {
		go u.notifyNewMember(userRes.GetUser().GetUsername(), userID)
	}

	return nil
}

func (u *chatUsecase) notifyNewMember(username, userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = u.authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
		Title:        "🎉 New member!",
		Body:         fmt.Sprintf("%s just joined the Phonkdrift community!", username),
		TargetUserId: "", // broadcast to all existing members
		DataType:     "community_join",
		DataId:       userID,
	})
}

func (u *chatUsecase) IsCommunityMember(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user_id is required")
	}
	return u.repo.IsCommunityMember(ctx, userID)
}

func (u *chatUsecase) SendMessage(ctx context.Context, userID, content, mediaURL, messageType, replyToID string) (*domain.ChatMessage, error) {
	isMember, err := u.repo.IsCommunityMember(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify membership: %w", err)
	}
	if !isMember {
		return nil, domain.ErrNotCommunityMember
	}

	content = strings.TrimSpace(content)
	if len(content) > maxContentLength {
		content = content[:maxContentLength]
	}
	if messageType == "" {
		messageType = "text"
	}
	if messageType != "text" && messageType != "audio" {
		return nil, errors.New(`message_type must be "text" or "audio"`)
	}
	if content == "" && mediaURL == "" {
		return nil, errors.New("message must have content or media_url")
	}

	// Denormalize the sender's username/avatar at write time so reads never
	// need a cross-service call back to auth-service.
	userRes, err := u.authClient.GetUser(ctx, &authpb.GetUserRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to look up sender profile: %w", err)
	}

	var replyToMsg *domain.ChatMessage
	if replyToID != "" {
		replyToMsg, err = u.repo.GetMessageByID(ctx, replyToID)
		if err != nil {
			return nil, fmt.Errorf("replied-to message not found: %w", err)
		}
	}

	created, err := u.repo.CreateMessage(ctx, domain.ChatMessage{
		UserID:      userID,
		Username:    userRes.GetUser().GetUsername(),
		AvatarURL:   userRes.GetUser().GetAvatarUrl(),
		Content:     content,
		MediaURL:    mediaURL,
		MessageType: messageType,
		ReplyToID:   replyToID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Push notifications (FCM): a reply-to-someone-else gets a targeted
	// "X replied to you" push instead of the generic broadcast, so the
	// replied-to user doesn't get double-notified for the same message.
	if replyToMsg != nil {
		created.ReplyToUserID = replyToMsg.UserID
		created.ReplyToUsername = replyToMsg.Username
		created.ReplyToContentSnippet = snippet(replyToMsg.Content, 80)

		if replyToMsg.UserID != userID {
			go u.notifyReply(created, replyToMsg)
		} else {
			go u.notifyNewMessage(created)
		}
	} else {
		go u.notifyNewMessage(created)
	}

	// Realtime delivery (RabbitMQ fanout -> SubscribeToChat streams) always
	// happens regardless of the push-notification branch above — these are
	// two independent delivery paths (live viewers vs. backgrounded users).
	if u.broadcaster != nil {
		if err := u.broadcaster.Publish(ctx, created); err != nil {
			// Non-fatal: the message is already saved; realtime delivery is
			// best-effort — GetMessages will still surface it on next fetch.
			fmt.Printf("⚠️ failed to broadcast chat message %s: %v\n", created.ID, err)
		}
	}

	return created, nil
}

func (u *chatUsecase) notifyNewMessage(created *domain.ChatMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := created.Content
	if body == "" {
		body = "sent a voice clip"
	}

	_, _ = u.authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
		Title:        created.Username,
		Body:         body,
		TargetUserId: "", // broadcast to all community members
		DataType:     "chat_message",
		DataId:       created.ID,
	})
}

func (u *chatUsecase) notifyReply(created, original *domain.ChatMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body := created.Content
	if body == "" {
		body = "sent a voice clip"
	}

	_, _ = u.authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
		Title:        fmt.Sprintf("%s replied to you", created.Username),
		Body:         body,
		TargetUserId: original.UserID,
		DataType:     "chat_reply",
		DataId:       created.ID,
	})
}

func (u *chatUsecase) GetMessages(ctx context.Context, userID string, beforeTimestamp int64, limit int32) ([]*domain.ChatMessage, error) {
	isMember, err := u.repo.IsCommunityMember(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to verify membership: %w", err)
	}
	if !isMember {
		return nil, domain.ErrNotCommunityMember
	}

	if limit <= 0 {
		limit = 30
	}

	before := time.Now()
	if beforeTimestamp > 0 {
		before = time.Unix(beforeTimestamp, 0)
	}

	messages, err := u.repo.GetMessagesBefore(ctx, before, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}

	for _, m := range messages {
		if m.ReplyToID == "" {
			continue
		}
		original, err := u.repo.GetMessageByID(ctx, m.ReplyToID)
		if err == nil && original != nil {
			m.ReplyToUserID = original.UserID
			m.ReplyToUsername = original.Username
			m.ReplyToContentSnippet = snippet(original.Content, 80)
		}
	}

	// Reverse to chronological (oldest-first) order for display — the query
	// fetches DESC (most-recent-before-cursor first) for efficient paging.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

const (
	newMemberWindow   = 7 * 24 * time.Hour
	foundingMemberMax = 50
)

func (u *chatUsecase) GetCommunityStats(ctx context.Context, userID string) (int64, error) {
	isMember, err := u.repo.IsCommunityMember(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to verify membership: %w", err)
	}
	if !isMember {
		return 0, domain.ErrNotCommunityMember
	}

	return u.repo.CountCommunityMembers(ctx)
}

func (u *chatUsecase) GetCommunityMembers(ctx context.Context, userID string, page, limit int32) ([]*domain.CommunityMember, int64, error) {
	isMember, err := u.repo.IsCommunityMember(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to verify membership: %w", err)
	}
	if !isMember {
		return nil, 0, domain.ErrNotCommunityMember
	}

	if limit <= 0 {
		limit = 30
	}
	if page < 0 {
		page = 0
	}

	total, err := u.repo.CountCommunityMembers(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count members: %w", err)
	}

	members, err := u.repo.ListCommunityMembers(ctx, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list members: %w", err)
	}

	for _, m := range members {
		m.Badge = memberBadge(m)
	}

	return members, total, nil
}

// memberBadge derives a UI badge from join order/recency. "first" (founding
// member, permanent) takes precedence over "new" (time-based, expires after
// newMemberWindow) since it's the rarer, more special distinction.
func memberBadge(m *domain.CommunityMember) string {
	if m.JoinRank <= foundingMemberMax {
		return "first"
	}
	if time.Since(m.JoinedAt) <= newMemberWindow {
		return "new"
	}
	return ""
}

func snippet(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
