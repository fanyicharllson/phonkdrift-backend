package domain

import (
	"context"
	"time"
)

// Feedback represents a single user-submitted app rating/comment.
type Feedback struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Rating     int32     `json:"rating"`
	Comment    string    `json:"comment"`
	AppVersion string    `json:"app_version"`
	CreatedAt  time.Time `json:"created_at"`
}

// FeedbackRepository defines the database data-access expectations for feedback.
type FeedbackRepository interface {
	CreateFeedback(ctx context.Context, userID string, rating int32, comment, appVersion string) (*Feedback, error)
	ListFeedback(ctx context.Context, page, limit int32) ([]*Feedback, int64, error)
}
