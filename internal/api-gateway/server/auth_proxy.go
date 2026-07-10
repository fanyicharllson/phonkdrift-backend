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
	req.UserId = verifiedUserID(ctx)
	return s.AuthClient.UpdateProfile(ctx, req)
}

func (s *GatewayServer) UploadAvatar(ctx context.Context, req *authpb.UploadAvatarRequest) (*authpb.UploadAvatarResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.AuthClient.UploadAvatar(ctx, req)
}

func (s *GatewayServer) ChangePassword(ctx context.Context, req *authpb.ChangePasswordRequest) (*authpb.ChangePasswordResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.AuthClient.ChangePassword(ctx, req)
}

func (s *GatewayServer) UpdateUsername(ctx context.Context, req *authpb.UpdateUsernameRequest) (*authpb.UpdateUsernameResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.AuthClient.UpdateUsername(ctx, req)
}

func (s *GatewayServer) SubmitFeedback(ctx context.Context, req *authpb.SubmitFeedbackRequest) (*authpb.SubmitFeedbackResponse, error) {
	req.UserId = verifiedUserID(ctx)
	return s.AuthClient.SubmitFeedback(ctx, req)
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

func (s *GatewayServer) BanUser(ctx context.Context, req *authpb.BanUserRequest) (*authpb.BanUserResponse, error) {
	return s.AuthClient.BanUser(ctx, req)
}

func (s *GatewayServer) UnbanUser(ctx context.Context, req *authpb.UnbanUserRequest) (*authpb.UnbanUserResponse, error) {
	return s.AuthClient.UnbanUser(ctx, req)
}

func (s *GatewayServer) SendPushNotification(ctx context.Context, req *authpb.PushNotificationRequest) (*authpb.PushNotificationResponse, error) {
	return s.AuthClient.SendPushNotification(ctx, req)
}

func (s *GatewayServer) UpdateFCMToken(ctx context.Context, req *authpb.UpdateFCMTokenRequest) (*authpb.UpdateFCMTokenResponse, error) {
	return s.AuthClient.UpdateFCMToken(ctx, req)
}

func (s *GatewayServer) GetUserStatus(ctx context.Context, req *authpb.GetUserStatusRequest) (*authpb.GetUserStatusResponse, error) {
	return s.AuthClient.GetUserStatus(ctx, req)
}
