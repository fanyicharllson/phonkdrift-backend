package grpc

import (
	"context"
	"errors"

	"github.com/fanyicharllson/phonkdrift-backend/internal/auth-service/domain"
	
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth" 
	
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthGRPCHandler struct {
	authpb.UnimplementedAuthServiceServer // Fits perfectly now
	useCase                               domain.AuthUseCase
}

func NewAuthGRPCHandler(u domain.AuthUseCase) *AuthGRPCHandler {
	return &AuthGRPCHandler{
		useCase: u,
	}
}

func (h *AuthGRPCHandler) RegisterUser(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	domainReq := domain.RegisterReq{
		Username: req.GetUsername(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	}

	user, err := h.useCase.Register(ctx, domainReq)
	if err != nil {
		if errors.Is(err, errors.New("email configuration address already registered")) {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Return only the fields that actually exist in your generated proto struct!
	return &authpb.RegisterResponse{
		UserId:  user.ID, // Notice the capital 'U' and 'I' to match 'UserId' precisely
		Message: "Registration successful! Check your email for your verification code.",
	}, nil
}

func (h *AuthGRPCHandler) VerifyCode(ctx context.Context, req *authpb.VerifyRequest) (*authpb.VerifyResponse, error) {
	// Execute the core validation business constraints
	success, err := h.useCase.VerifyCode(ctx, req.GetEmail(), req.GetCode())
	if err != nil {
		// Return a clean gRPC status code error to the mobile app
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &authpb.VerifyResponse{
		Success: success,
		Message: "Your profile has been successfully verified! Welcome to PhonkDrift. ✓",
	}, nil
}