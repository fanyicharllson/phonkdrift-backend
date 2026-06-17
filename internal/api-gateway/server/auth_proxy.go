package server

import (
	"context"
	"strings"

	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"google.golang.org/grpc/metadata"
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
	if req.GetToken() == "" {
		req = &authpb.ValidateTokenRequest{
			Token: bearerTokenFromIncomingContext(ctx),
		}
	}
	return s.AuthClient.ValidateToken(ctx, req)
}

func (s *GatewayServer) ResendCode(ctx context.Context, req *authpb.ResendCodeRequest) (*authpb.ResendCodeResponse, error) {
	return s.AuthClient.ResendCode(ctx, req)
}

func (s *GatewayServer) GetUser(ctx context.Context, req *authpb.GetUserRequest) (*authpb.GetUserResponse, error) {
	return s.AuthClient.GetUser(ctx, req)
}

func (s *GatewayServer) UpdateProfile(ctx context.Context, req *authpb.UpdateProfileRequest) (*authpb.UpdateProfileResponse, error) {
	return s.AuthClient.UpdateProfile(ctx, req)
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

func bearerTokenFromIncomingContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	for _, value := range md.Get("authorization") {
		parts := strings.Fields(strings.TrimSpace(value))
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	return ""
}
