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
	PhonkLevel   string    `json:"phonk_level"`
	IsVerified   bool      `json:"is_verified"`
	IsBanned     bool      `json:"is_banned"`
	FCMToken     string    `json:"fcm_token"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
    BanReason string `json:"ban_reason"`
}

// RegisterReq defines the explicit parameters required to invoke registration
type RegisterReq struct {
	Username string
	Email    string
	Password string
}

type VerificationDetails struct {
	UserID           string
	VerificationCode string
	CodeExpiresAt    time.Time
	IsVerified       bool
}

// AuthRepository defines the database data-access expectations (Hexagonal Output Port)
type AuthRepository interface {
	CreateUser(ctx context.Context, username, email, passwordHash, vCode string, expiresAt time.Time) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetVerificationDetails(ctx context.Context, email string) (*VerificationDetails, error)
	MarkUserVerified(ctx context.Context, userID string) error
	UpdateUserVerificationCode(ctx context.Context, email, vCode string, expiresAt time.Time) error
	GetUserByID(ctx context.Context, userID string) (*User, error)
	UpdateUserPhonkLevel(ctx context.Context, userID, phonkLevel string) (*User, error)
	UpdatePassword(ctx context.Context, userID string, hashedPassword string) error
	BanUser(ctx context.Context, userID string, reason string) error
	UnbanUser(ctx context.Context, userID string) error
	UpdateFCMToken(ctx context.Context, userID string, fcmToken string) error
	GetAllFCMTokens(ctx context.Context) ([]string, error)
	CountUsers(ctx context.Context) (int64, error)

}

// EventEventPublisher defines the expectations for queuing async tasks (Hexagonal Output Port)
type EventPublisher interface {
	PublishUserRegistered(ctx context.Context, username, email, verificationCode string) error
	PublishUserVerified(ctx context.Context, username, email string) error
}

// AuthUseCase defines the core business orchestration entry point (Hexagonal Input Port)
type AuthUseCase interface {
	Register(ctx context.Context, req RegisterReq) (*User, error)
	VerifyCode(ctx context.Context, email, code string) (string, int64, *User, error)
	LoginUser(ctx context.Context, email, password string) (string, *User, int64, error)
	ValidateToken(ctx context.Context, tokenString string) (string, string, error)
	ResendCode(ctx context.Context, email string) error
	GetUser(ctx context.Context, userID string) (*User, error)
	UpdateProfile(ctx context.Context, userID, phonkLevel string) (*User, error)
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, email, code, newPassword string) error
	VerifyResetCode(ctx context.Context, email, code string) (bool, error)
	BanUser(ctx context.Context, userID string, reason string) error
	UnbanUser(ctx context.Context, userID string) error
	UpdateFCMToken(ctx context.Context, userID string, fcmToken string) error
	GetUserStatus(ctx context.Context, userID string) (*User, error)
	SendPushNotification(ctx context.Context, title, body, targetUserID, dataType, dataID string) (int, error)
	GetUserCount(ctx context.Context) (int64, error)
}
