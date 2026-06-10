package repository

import (
	"context"
	"database/sql"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/repository/db"
)

type authRepository struct {
	queries *db.Queries
}

// NewAuthRepository binds our sqlc generated code to our domain interface
func NewAuthRepository(sqlDB *sql.DB) domain.AuthRepository {
	return &authRepository{
		queries: db.New(sqlDB),
	}
}

func (r *authRepository) CreateUser(ctx context.Context, username, email, passwordHash string) (*domain.User, error) {
	sqlcUser, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:           sqlcUser.ID.String(),
		Username:     sqlcUser.Username,
		Email:        sqlcUser.Email,
		AvatarURL:    sqlcUser.AvatarUrl.String,
		IsVerified:   sqlcUser.IsVerified.Bool,
		CreatedAt:    sqlcUser.CreatedAt.Time,
		UpdatedAt:    sqlcUser.UpdatedAt.Time,
	}, nil
}

func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	sqlcUser, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return clean nil if user doesn't exist yet
		}
		return nil, err
	}

	return &domain.User{
		ID:           sqlcUser.ID.String(),
		Username:     sqlcUser.Username,
		Email:        sqlcUser.Email,
		PasswordHash: sqlcUser.PasswordHash,
		AvatarURL:    sqlcUser.AvatarUrl.String,
		IsVerified:   sqlcUser.IsVerified.Bool,
		CreatedAt:    sqlcUser.CreatedAt.Time,
		UpdatedAt:    sqlcUser.UpdatedAt.Time,
	}, nil
}