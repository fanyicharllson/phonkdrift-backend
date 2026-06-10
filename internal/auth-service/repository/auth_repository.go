package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/repository/db"

	"github.com/google/uuid"
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

func (r *authRepository) CreateUser(ctx context.Context, username, email, passwordHash, vCode string, expiresAt time.Time) (*domain.User, error) {
	sqlcUser, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		Username:         username,
		Email:            email,
		PasswordHash:     passwordHash,
		VerificationCode: sql.NullString{String: vCode, Valid: true},
		CodeExpiresAt:    sql.NullTime{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &domain.User{
		ID:         sqlcUser.ID.String(),
		Username:   sqlcUser.Username,
		Email:      sqlcUser.Email,
		AvatarURL:  sqlcUser.AvatarUrl.String,
		IsVerified: sqlcUser.IsVerified.Bool,
		CreatedAt:  sqlcUser.CreatedAt.Time,
		UpdatedAt:  sqlcUser.UpdatedAt.Time,
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

func (r *authRepository) GetVerificationDetails(ctx context.Context, email string) (*domain.VerificationDetails, error) {
	details, err := r.queries.GetVerificationDetails(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &domain.VerificationDetails{
		UserID:           details.ID.String(),
		VerificationCode: details.VerificationCode.String,
		CodeExpiresAt:    details.CodeExpiresAt.Time,
		IsVerified:       details.IsVerified.Bool,
	}, nil
}

func (r *authRepository) MarkUserVerified(ctx context.Context, userID string) error {
	parsedUUID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}

	// Now that fields are named explicitly, they will map perfectly!
	_, err = r.queries.UpdateUserVerification(ctx, db.UpdateUserVerificationParams{
		IsVerified: true,       // Native bool from @is_verified
		ID:         parsedUUID, // uuid.UUID from @id
	})
	return err
}

func (r *authRepository) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	parsedUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	user, err := r.queries.GetUserByID(ctx, parsedUUID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &domain.User{
		ID:         user.ID.String(),
		Username:   user.Username,
		Email:      user.Email,
		AvatarURL:  user.AvatarUrl.String,
		IsVerified: user.IsVerified.Bool,
		CreatedAt:  user.CreatedAt.Time,
		UpdatedAt:  user.UpdatedAt.Time,
	}, nil
}

func (r *authRepository) UpdateUserVerificationCode(ctx context.Context, email, vCode string, expiresAt time.Time) error {
	return r.queries.UpdateUserVerificationCode(ctx, db.UpdateUserVerificationCodeParams{
		Email:            email,
		VerificationCode: vCode,
		CodeExpiresAt:    expiresAt,
	})
}
