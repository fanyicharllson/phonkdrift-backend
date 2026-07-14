package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotCommunityMember is returned when a user tries to read/send/subscribe
// to chat without having joined the community first.
var ErrNotCommunityMember = errors.New("must join the community first")

// ChatMessage represents a single message in the global community room.
type ChatMessage struct {
	ID                    string
	UserID                string
	Username              string
	AvatarURL             string
	Content               string
	MediaURL              string
	MessageType           string
	ReplyToID             string
	ReplyToUserID         string
	ReplyToUsername       string
	ReplyToContentSnippet string
	CreatedAt             time.Time
}

// CommunityMember represents a single row in the member roster, with a
// UI-facing badge derived from join order/recency.
type CommunityMember struct {
	UserID    string
	Username  string
	AvatarURL string
	JoinedAt  time.Time
	JoinRank  int64
	Badge     string // "first" | "new" | "" — populated by the usecase layer
}

// ChatRepository defines the database data-access expectations (Hexagonal Output Port)
type ChatRepository interface {
	// JoinCommunity returns true if this call actually inserted a new
	// membership row (false if the user was already a member).
	JoinCommunity(ctx context.Context, userID, username, avatarURL string) (bool, error)
	IsCommunityMember(ctx context.Context, userID string) (bool, error)
	CountCommunityMembers(ctx context.Context) (int64, error)
	ListCommunityMembers(ctx context.Context, page, limit int32) ([]*CommunityMember, error)
	CreateMessage(ctx context.Context, msg ChatMessage) (*ChatMessage, error)
	GetMessageByID(ctx context.Context, id string) (*ChatMessage, error)
	GetMessagesBefore(ctx context.Context, before time.Time, limit int32) ([]*ChatMessage, error)
}

// ChatUsecase defines the core business orchestration entry point (Hexagonal Input Port)
type ChatUsecase interface {
	JoinCommunity(ctx context.Context, userID string) error
	IsCommunityMember(ctx context.Context, userID string) (bool, error)
	SendMessage(ctx context.Context, userID, content, mediaURL, messageType, replyToID string) (*ChatMessage, error)
	GetMessages(ctx context.Context, userID string, beforeTimestamp int64, limit int32) ([]*ChatMessage, error)
	GetCommunityStats(ctx context.Context, userID string) (int64, error)
	GetCommunityMembers(ctx context.Context, userID string, page, limit int32) ([]*CommunityMember, int64, error)
}
