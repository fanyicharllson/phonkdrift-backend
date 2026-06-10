package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
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

	// 🚀 STEP 4: GENERATE VERIFICATION DETAILS FIRST
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	vCode := fmt.Sprintf("%06d", rng.Intn(1000000))
	expiresAt := time.Now().Add(15 * time.Minute) // 15-Minute Expiration Window Target

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

// Make sure the method name, receiver (*authUseCase), and arguments match perfectly:
func (u *authUseCase) VerifyCode(ctx context.Context, email, code string) (bool, error) {
	details, err := u.repo.GetVerificationDetails(ctx, email)
	if err != nil || details == nil {
		return false, errors.New("user account profile registration parameters not found")
	}

	if details.IsVerified {
		return true, nil
	}

	if time.Now().After(details.CodeExpiresAt) {
		return false, errors.New("verification security token transaction has expired (15 min window reached)")
	}

	if details.VerificationCode != code {
		return false, errors.New("invalid verification security code sequence provided")
	}

	err = u.repo.MarkUserVerified(ctx, details.UserID)
	if err != nil {
		return false, errors.New("failed to upgrade user profile activation state")
	}

	// DISPATCH THE WELCOME EVENT IN THE BACKGROUND:
	userProfile, _ := u.repo.GetUserByEmail(ctx, email)
	if userProfile != nil {
		_ = u.publisher.PublishUserVerified(ctx, userProfile.Username, email)
	}

	return true, nil
}