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

	// 4. Commit user to database layer (IsVerified defaults to false)
	newUser, err := u.repo.CreateUser(ctx, username, email, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to write profile transaction: %w", err)
	}

	// 5. Generate a random 6-digit numeric verification code
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	vCode := fmt.Sprintf("%06d", rng.Intn(1000000))

	// 6. Push event payload asynchronously into RabbitMQ pipeline
	err = u.publisher.PublishUserRegistered(ctx, newUser.Username, newUser.Email, vCode)
	if err != nil {
		// Log warning but don't crash registration; worker can pick up or retry mechanism can be implemented
		fmt.Printf("Asynchronous pipeline delivery warning: %v\n", err)
	}

	return newUser, nil
}