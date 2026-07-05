package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type authUseCase struct {
	repo      domain.AuthRepository
	publisher domain.EventPublisher
}

// NewAuthUseCase instantiates the business logic block with required output ports
func NewAuthUseCase(r domain.AuthRepository, p domain.EventPublisher) domain.AuthUseCase {
	return &authUseCase{
		repo:      r,
		publisher: p,
	}
}

func (u *authUseCase) Register(ctx context.Context, req domain.RegisterReq) (*domain.User, error) {
	// 1. Basic sanity sanitization
	email := strings.ToLower(strings.TrimSpace(req.Email))
	username := strings.TrimSpace(req.Username)

	if email == "" || username == "" || len(req.Password) < 6 {
		return nil, errors.New("invalid registration credentials: password must be at least 6 characters")
	}

	// 2. Check if user already exists
	existingUser, _ := u.repo.GetUserByEmail(ctx, email)
	if existingUser != nil {
		return nil, errors.New("email configuration address already registered")
	}

	// 3. Securely hash incoming plain password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to safely secure user secret: %w", err)
	}

	vCode, expiresAt := generateCode()

	// 🚀 STEP 5: COMMIT EVERYTHING TO DATABASE LAYER (Now passing vCode and expiresAt)
	newUser, err := u.repo.CreateUser(ctx, username, email, string(hashedPassword), vCode, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to write profile transaction: %w", err)
	}

	// 6. Push event payload asynchronously into RabbitMQ pipeline
	err = u.publisher.PublishUserRegistered(ctx, newUser.Username, newUser.Email, vCode)
	if err != nil {
		fmt.Printf("Asynchronous pipeline delivery warning: %v\n", err)
	}

	return newUser, nil
}

func (u *authUseCase) VerifyCode(ctx context.Context, email, code string) (string, int64, *domain.User, error) {
	details, err := u.repo.GetVerificationDetails(ctx, email)
	if err != nil || details == nil {
		return "", 0, nil, errors.New("user account profile registration parameters not found")
	}

	if details.IsVerified {
		return "", 0, nil, errors.New("account is already verified")
	}

	if time.Now().After(details.CodeExpiresAt) {
		return "", 0, nil, errors.New("verification security token transaction has expired")
	}

	if details.VerificationCode != code {
		return "", 0, nil, errors.New("invalid verification security code sequence provided")
	}

	err = u.repo.MarkUserVerified(ctx, details.UserID)
	if err != nil {
		return "", 0, nil, errors.New("failed to upgrade user profile activation state")
	}

	userProfile, err := u.repo.GetUserByID(ctx, details.UserID)
	if err != nil || userProfile == nil {
		return "", 0, nil, errors.New("failed to retrieve verified profile info")
	}

	// Clean centralized call 🏎️💨
	tokenString, expiresAt, err := generateAccessToken(userProfile.ID, userProfile.Username)
	if err != nil {
		return "", 0, nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	_ = u.publisher.PublishUserVerified(ctx, userProfile.Username, email)

	return tokenString, expiresAt, userProfile, nil
}

func (u *authUseCase) LoginUser(ctx context.Context, email, password string) (string, *domain.User, int64, error) {
	user, err := u.repo.GetUserByEmail(ctx, strings.ToLower(email))
	if err != nil || user == nil {
		return "", nil, 0, errors.New("invalid email or password")
	}
	if !user.IsVerified {
		return "", nil, 0, errors.New("please verify your email before logging in")
	}

	if user.IsBanned {
		return "", nil, 0, fmt.Errorf("account suspended: %s", user.BanReason)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", nil, 0, errors.New("invalid email or password")
	}

	// Clean centralized call 🏎️💨
	tokenString, expiresAt, err := generateAccessToken(user.ID, user.Username)
	if err != nil {
		return "", nil, 0, fmt.Errorf("failed to generate access token: %w", err)
	}

	return tokenString, user, expiresAt, nil
}

func (u *authUseCase) ValidateToken(ctx context.Context, tokenString string) (string, string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "phonk-drift-default-secret-key"
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return "", "", errors.New("session expired or invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", errors.New("failed to parse session identity")
	}
	expiresAt, err := claims.GetExpirationTime()
	if err != nil || expiresAt == nil || time.Now().After(expiresAt.Time) {
		return "", "", errors.New("session expired or invalid token")
	}

	userID, _ := claims["user_id"].(string)
	username, _ := claims["username"].(string)
	if userID == "" {
		return "", "", errors.New("failed to parse session identity")
	}

	user, err := u.repo.GetUserByID(ctx, userID)
	if err == nil && user != nil && user.IsBanned {
		return "", "", errors.New("account suspended")
	}

	return userID, username, nil
}

func (u *authUseCase) ResendCode(ctx context.Context, email string) error {
	user, err := u.repo.GetUserByEmail(ctx, strings.ToLower(email))
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	if user.IsVerified {
		return errors.New("account is already verified")
	}

	// Dynamic 6 digit generator syntax secure block
	vCode, expiresAt := generateCode()

	err = u.repo.UpdateUserVerificationCode(ctx, user.Email, vCode, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to refresh security code: %w", err)
	}

	_ = u.publisher.PublishUserRegistered(ctx, user.Username, user.Email, vCode)
	return nil
}

func (u *authUseCase) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	user, err := u.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user profile not found")
	}
	return user, nil
}

func (u *authUseCase) GetUserStatus(ctx context.Context, userID string) (*domain.User, error) {
	user, err := u.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (u *authUseCase) UpdateProfile(ctx context.Context, userID, phonkLevel string) (*domain.User, error) {
	userID = strings.TrimSpace(userID)
	phonkLevel = strings.ToUpper(strings.TrimSpace(phonkLevel))
	if userID == "" {
		return nil, errors.New("user_id is required")
	}
	if phonkLevel == "" {
		return nil, errors.New("phonk_level is required")
	}

	user, err := u.repo.UpdateUserPhonkLevel(ctx, userID, phonkLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}
	if user == nil {
		return nil, errors.New("user profile not found")
	}

	return user, nil
}

func (u *authUseCase) ForgotPassword(ctx context.Context, email string) error {
	user, err := u.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || user == nil {
		return nil
	}

	vCode, expiresAt := generateCode()

	// Reuse repository method to save the fresh code and push out the expiration window
	err = u.repo.UpdateUserVerificationCode(ctx, user.Email, vCode, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	_ = u.publisher.PublishUserRegistered(ctx, user.Username, user.Email, vCode)

	return nil
}

func (u *authUseCase) ResetPassword(ctx context.Context, email, code, newPassword string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(newPassword) < 6 {
		return errors.New("password must be at least 6 characters long")
	}

	details, err := u.repo.GetVerificationDetails(ctx, email)
	if err != nil || details == nil {
		return errors.New("verification details not found")
	}

	if time.Now().After(details.CodeExpiresAt) {
		return errors.New("reset code has expired")
	}

	if details.VerificationCode != code {
		return errors.New("invalid reset code")
	}

	// Securely hash the new incoming password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = u.repo.UpdatePassword(ctx, details.UserID, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to save new password: %w", err)
	}

	return nil
}

func (u *authUseCase) VerifyResetCode(ctx context.Context, email, code string) (bool, error) {
	details, err := u.repo.GetVerificationDetails(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || details == nil {
		return false, errors.New("reset details not found")
	}

	if time.Now().After(details.CodeExpiresAt) {
		return false, errors.New("reset code has expired")
	}

	if details.VerificationCode != code {
		return false, errors.New("invalid verification code")
	}

	return true, nil
}

func (u *authUseCase) BanUser(ctx context.Context, userID, reason string) error {
	return u.repo.BanUser(ctx, userID, reason)
}

func (u *authUseCase) UnbanUser(ctx context.Context, userID string) error {
	return u.repo.UnbanUser(ctx, userID)
}

func (u *authUseCase) GetUserCount(ctx context.Context) (int64, error) {
	return u.repo.CountUsers(ctx)
}

func (u *authUseCase) UpdateFCMToken(ctx context.Context, userID, fcmToken string) error {
	return u.repo.UpdateFCMToken(ctx, userID, fcmToken)
}

func (u *authUseCase) SendPushNotification(ctx context.Context, title, body, targetUserID, dataType, dataID string) (int, error) {
	var tokens []string
	var err error

	if targetUserID != "" {
		// Single user push
		user, err := u.repo.GetUserByID(ctx, targetUserID)
		if err != nil || user == nil {
			return 0, fmt.Errorf("target user not found")
		}
		if user.FCMToken == "" {
			return 0, fmt.Errorf("user has no FCM token registered")
		}
		tokens = []string{user.FCMToken}
	} else {
		// Broadcast to all active users
		tokens, err = u.repo.GetAllFCMTokens(ctx)
		if err != nil {
			return 0, fmt.Errorf("failed to fetch FCM tokens: %w", err)
		}
	}

	if len(tokens) == 0 {
		return 0, nil
	}

	sentCount, err := sendFCMNotifications(tokens, title, body, dataType, dataID)
	if err != nil {
		return 0, err
	}

	return sentCount, nil
}


// Helpers
func generateCode() (string, time.Time) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	vCode := fmt.Sprintf("%06d", rng.Intn(1000000))
	expiresAt := time.Now().Add(15 * time.Minute)

	return vCode, expiresAt
}

// Reusable token generation utility helper
func generateAccessToken(userID, username string) (string, int64, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "phonk-drift-default-secret-key"
	}

	expiresAt := time.Now().Add(72 * time.Hour).Unix()
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"exp":      expiresAt,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", 0, err
	}

	return tokenString, expiresAt, nil
}
