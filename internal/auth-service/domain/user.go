package domain

import (
	"context"
	"time"
)

// User represents the core domain entity for a PhonkDrift account
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	AvatarURL    string    `json:"avatar_url"`
	IsVerified   bool      `json:"is_verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RegisterReq defines the explicit parameters required to invoke registration
type RegisterReq struct {
	Username string
	Email    string
	Password string
}

// AuthRepository defines the database data-access expectations (Hexagonal Output Port)
type AuthRepository interface {
	CreateUser(ctx context.Context, username, email, passwordHash string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

// EventEventPublisher defines the expectations for queuing async tasks (Hexagonal Output Port)
type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, username, email, verificationCode string) error
}

// AuthUseCase defines the core business orchestration entry point (Hexagonal Input Port)
type AuthUseCase interface {
	Register(ctx context.Context, req RegisterReq) (*User, error)
}