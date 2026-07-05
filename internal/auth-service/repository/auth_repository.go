package repository

import (
	"context"
	"database/sql"
	"fmt"
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

	return mapSQLCUser(db.User{
		ID:         sqlcUser.ID,
		Username:   sqlcUser.Username,
		Email:      sqlcUser.Email,
		AvatarUrl:  sqlcUser.AvatarUrl,
		IsVerified: sqlcUser.IsVerified,
		CreatedAt:  sqlcUser.CreatedAt,
		UpdatedAt:  sqlcUser.UpdatedAt,
	}), nil
}

func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	sqlcUser, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return clean nil if user doesn't exist yet
		}
		return nil, err
	}

	return mapSQLCUser(sqlcUser), nil
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

	return mapSQLCUser(user), nil
}

func (r *authRepository) UpdateUserVerificationCode(ctx context.Context, email, vCode string, expiresAt time.Time) error {
	return r.queries.UpdateUserVerificationCode(ctx, db.UpdateUserVerificationCodeParams{
		Email:            email,
		VerificationCode: vCode,
		CodeExpiresAt:    expiresAt,
	})
}

func (r *authRepository) UpdatePassword(ctx context.Context, userID string, hashedPassword string) error {
	parsedUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user uuid structure: %w", err)
	}

	return r.queries.UpdatePassword(ctx, db.UpdatePasswordParams{
		ID:           parsedUUID,
		PasswordHash: hashedPassword,
	})
}

func (r *authRepository) UpdateUserPhonkLevel(ctx context.Context, userID, phonkLevel string) (*domain.User, error) {
	parsedUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user uuid structure: %w", err)
	}

	user, err := r.queries.UpdateUserPhonkLevel(ctx, db.UpdateUserPhonkLevelParams{
		ID:         parsedUUID,
		PhonkLevel: sql.NullString{String: phonkLevel, Valid: true},
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return mapSQLCUser(user), nil
}

func mapSQLCUser(user db.User) *domain.User {
	return &domain.User{
		ID:           user.ID.String(),
		Username:     user.Username,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		AvatarURL:    user.AvatarUrl.String,
		PhonkLevel:   user.PhonkLevel.String,
		IsVerified:   user.IsVerified.Bool,
		IsBanned:     user.IsBanned,
		BanReason: func() string {
			if user.BanReason.Valid {
				return user.BanReason.String
			}
			return ""
		}(),
		FCMToken: func() string {
			if user.FcmToken.Valid {
				return user.FcmToken.String
			}
			return ""
		}(),
		// FCMToken:  user.FcmToken.String,
		CreatedAt: user.CreatedAt.Time,
		UpdatedAt: user.UpdatedAt.Time,
	}
}

func (r *authRepository) BanUser(ctx context.Context, userID, reason string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	return r.queries.BanUser(ctx, db.BanUserParams{
		ID:        uid,
		BanReason: sql.NullString{String: reason, Valid: reason != ""},
	})
}

func (r *authRepository) UnbanUser(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	return r.queries.UnbanUser(ctx, uid)
}

func (r *authRepository) UpdateFCMToken(ctx context.Context, userID, fcmToken string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}
	return r.queries.UpdateFCMToken(ctx, db.UpdateFCMTokenParams{
		ID:       uid,
		FcmToken: sql.NullString{String: fcmToken, Valid: fcmToken != ""},
	})
}

func (r *authRepository) CountUsers(ctx context.Context) (int64, error) {
	return r.queries.CountUsers(ctx)
}

func (r *authRepository) GetAllFCMTokens(ctx context.Context) ([]string, error) {
	rows, err := r.queries.GetUserFCMTokens(ctx)
	if err != nil {
		return nil, err
	}
	var tokens []string
	for _, row := range rows {
		if row.Valid && row.String != "" {
			tokens = append(tokens, row.String)
		}
	}
	return tokens, nil
}
