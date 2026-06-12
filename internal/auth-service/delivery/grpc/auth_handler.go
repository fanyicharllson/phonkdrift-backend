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
	// Capture all 4 return parameters from our updated use case flow 🏎️💨
	token, expiresAt, user, err := h.useCase.VerifyCode(ctx, req.GetEmail(), req.GetCode())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &authpb.VerifyResponse{
		Success:   true,
		Message:   "Your profile has been successfully verified! Welcome to PhonkDrift. ✓",
		Token:     token,
		ExpiresAt: expiresAt,
		UserId:    user.ID,
	}, nil
}

func (h *AuthGRPCHandler) LoginUser(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	token, user, expiresAt, err := h.useCase.LoginUser(ctx, req.GetEmail(), req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &authpb.LoginResponse{
		Token:     token,
		UserId:    user.ID,
		ExpiresAt: expiresAt,
	}, nil
}

func (h *AuthGRPCHandler) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	userID, username, err := h.useCase.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return &authpb.ValidateTokenResponse{
		IsValid:  true,
		UserId:   userID,
		Username: username,
	}, nil
}

func (h *AuthGRPCHandler) ResendCode(ctx context.Context, req *authpb.ResendCodeRequest) (*authpb.ResendCodeResponse, error) {
	err := h.useCase.ResendCode(ctx, req.GetEmail())
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err.Error() == "account is already verified" {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Error(codes.Internal, "failed to resend code: "+err.Error())
	}
	return &authpb.ResendCodeResponse{
		Success: true,
		Message: "Verification code resent successfully. Check your email.",
	}, nil
}

func (h *AuthGRPCHandler) GetUser(ctx context.Context, req *authpb.GetUserRequest) (*authpb.GetUserResponse, error) {
	user, err := h.useCase.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &authpb.GetUserResponse{
		User: &authpb.User{
			UserId:    user.ID,
			Username:  user.Username,
			Email:     user.Email,
			AvatarUrl: user.AvatarURL,
		},
	}, nil
}

func (h *AuthGRPCHandler) ForgotPassword(ctx context.Context, req *authpb.ForgotPasswordRequest) (*authpb.ForgotPasswordResponse, error) {
	err := h.useCase.ForgotPassword(ctx, req.GetEmail())
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &authpb.ForgotPasswordResponse{
		Success: true,
		Message: "Password reset code sent successfully to your email.",
	}, nil
}

func (h *AuthGRPCHandler) ResetPassword(ctx context.Context, req *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
	err := h.useCase.ResetPassword(ctx, req.GetEmail(), req.GetCode(), req.GetNewPassword())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &authpb.ResetPasswordResponse{
		Success: true,
		Message: "Your password has been reset successfully. You can now login.",
	}, nil
}

func (h *AuthGRPCHandler) VerifyResetCode(ctx context.Context, req *authpb.VerifyResetCodeRequest) (*authpb.VerifyResetCodeResponse, error) {
	success, err := h.useCase.VerifyResetCode(ctx, req.GetEmail(), req.GetCode())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &authpb.VerifyResetCodeResponse{
		Success: success,
	}, nil
}