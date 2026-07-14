package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/domain"
	"github.com/fanyicharllson/phonkdrift-backend/internal/chat-service/repository/db"

	"github.com/google/uuid"
)

type chatRepository struct {
	queries *db.Queries
}

// NewChatRepository binds our sqlc generated code to our domain interface
func NewChatRepository(sqlDB *sql.DB) domain.ChatRepository {
	return &chatRepository{queries: db.New(sqlDB)}
}

func (r *chatRepository) JoinCommunity(ctx context.Context, userID, username, avatarURL string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}

	_, err = r.queries.JoinCommunity(ctx, db.JoinCommunityParams{
		UserID:    uid,
		Username:  username,
		AvatarUrl: sql.NullString{String: avatarURL, Valid: avatarURL != ""},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil // already a member — ON CONFLICT DO NOTHING, no row returned
		}
		return false, err
	}

	return true, nil
}

func (r *chatRepository) LeaveCommunity(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	return r.queries.LeaveCommunity(ctx, uid)
}

func (r *chatRepository) IsCommunityMember(ctx context.Context, userID string) (bool, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	return r.queries.IsCommunityMember(ctx, uid)
}

func (r *chatRepository) CountCommunityMembers(ctx context.Context) (int64, error) {
	return r.queries.CountCommunityMembers(ctx)
}

func (r *chatRepository) ListCommunityMembers(ctx context.Context, page, limit int32) ([]*domain.CommunityMember, error) {
	rows, err := r.queries.ListCommunityMembers(ctx, db.ListCommunityMembersParams{
		Limit:  limit,
		Offset: page * limit,
	})
	if err != nil {
		return nil, err
	}

	members := make([]*domain.CommunityMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, &domain.CommunityMember{
			UserID:    row.UserID.String(),
			Username:  row.Username,
			AvatarURL: row.AvatarUrl.String,
			JoinedAt:  row.JoinedAt.Time,
			JoinRank:  row.JoinRank,
		})
	}
	return members, nil
}

func (r *chatRepository) CreateMessage(ctx context.Context, msg domain.ChatMessage) (*domain.ChatMessage, error) {
	uid, err := uuid.Parse(msg.UserID)
	if err != nil {
		return nil, err
	}

	var replyTo uuid.NullUUID
	if msg.ReplyToID != "" {
		parsed, err := uuid.Parse(msg.ReplyToID)
		if err != nil {
			return nil, err
		}
		replyTo = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	row, err := r.queries.CreateMessage(ctx, db.CreateMessageParams{
		UserID:      uid,
		Username:    msg.Username,
		AvatarUrl:   sql.NullString{String: msg.AvatarURL, Valid: msg.AvatarURL != ""},
		Content:     sql.NullString{String: msg.Content, Valid: msg.Content != ""},
		MediaUrl:    sql.NullString{String: msg.MediaURL, Valid: msg.MediaURL != ""},
		MessageType: msg.MessageType,
		ReplyToID:   replyTo,
	})
	if err != nil {
		return nil, err
	}

	return mapSQLCMessage(row), nil
}

func (r *chatRepository) GetMessageByID(ctx context.Context, id string) (*domain.ChatMessage, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	row, err := r.queries.GetMessageByID(ctx, uid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return mapSQLCMessage(row), nil
}

func (r *chatRepository) GetMessagesBefore(ctx context.Context, before time.Time, limit int32) ([]*domain.ChatMessage, error) {
	rows, err := r.queries.GetMessagesBefore(ctx, db.GetMessagesBeforeParams{
		CreatedAt: sql.NullTime{Time: before, Valid: true},
		Limit:     limit,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]*domain.ChatMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, mapSQLCMessage(row))
	}
	return messages, nil
}

func mapSQLCMessage(m db.ChatMessage) *domain.ChatMessage {
	var replyToID string
	if m.ReplyToID.Valid {
		replyToID = m.ReplyToID.UUID.String()
	}

	return &domain.ChatMessage{
		ID:          m.ID.String(),
		UserID:      m.UserID.String(),
		Username:    m.Username,
		AvatarURL:   m.AvatarUrl.String,
		Content:     m.Content.String,
		MediaURL:    m.MediaUrl.String,
		MessageType: m.MessageType,
		ReplyToID:   replyToID,
		CreatedAt:   m.CreatedAt.Time,
	}
}
