package server

import (
	"context"

	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
)

// RegisterUser wraps and forwards the registration stream to the internal Auth microservice.
func (s *GatewayServer) RegisterUser(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	return s.AuthClient.RegisterUser(ctx, req)
}

func (s *GatewayServer) LoginUser(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	return s.AuthClient.LoginUser(ctx, req)
}

func (s *GatewayServer) VerifyCode(ctx context.Context, req *authpb.VerifyRequest) (*authpb.VerifyResponse, error) {
	return s.AuthClient.VerifyCode(ctx, req)
}

func (s *GatewayServer) ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	return s.AuthClient.ValidateToken(ctx, req)
}

func (s *GatewayServer) ResendCode(ctx context.Context, req *authpb.ResendCodeRequest) (*authpb.ResendCodeResponse, error) {
	return s.AuthClient.ResendCode(ctx, req)
}

func (s *GatewayServer) GetUser(ctx context.Context, req *authpb.GetUserRequest) (*authpb.GetUserResponse, error) {
	return s.AuthClient.GetUser(ctx, req)
}

func (s *GatewayServer) ForgotPassword(ctx context.Context, req *authpb.ForgotPasswordRequest) (*authpb.ForgotPasswordResponse, error) {
	return s.AuthClient.ForgotPassword(ctx, req)
}

func (s *GatewayServer) VerifyResetCode(ctx context.Context, req *authpb.VerifyResetCodeRequest) (*authpb.VerifyResetCodeResponse, error) {
	return s.AuthClient.VerifyResetCode(ctx, req)
}

func (s *GatewayServer) ResetPassword(ctx context.Context, req *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
	return s.AuthClient.ResetPassword(ctx, req)
}