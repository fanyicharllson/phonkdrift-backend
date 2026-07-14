package server

import (
	"context"
	"strings"

	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDContextKey contextKey = "user_id"

var publicMethods = map[string]struct{}{
	"/auth.AuthService/Login":    {},
	"/auth.AuthService/Register": {},
	// "/auth.AuthService/ForgotPassword": {},

	// Current proto method names.
	authpb.AuthService_LoginUser_FullMethodName:    {},
	authpb.AuthService_RegisterUser_FullMethodName: {},

	authpb.AuthService_VerifyCode_FullMethodName:      {},
	authpb.AuthService_ResendCode_FullMethodName:      {},
	authpb.AuthService_ForgotPassword_FullMethodName:  {},
	authpb.AuthService_VerifyResetCode_FullMethodName: {},
	authpb.AuthService_ResetPassword_FullMethodName:   {},
	authpb.AuthService_GetUserStatus_FullMethodName:   {},
}

func (s *GatewayServer) authUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if _, ok := publicMethods[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		token, ok := bearerTokenFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "authorization bearer token missing")
		}

		res, err := s.AuthClient.ValidateToken(ctx, &authpb.ValidateTokenRequest{
			Token: token,
		})
		if err != nil || !res.GetIsValid() || res.GetUserId() == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired authorization token")
		}

		ctx = context.WithValue(ctx, userIDContextKey, res.GetUserId())
		ctx = metadata.AppendToOutgoingContext(ctx, "user_id", res.GetUserId())

		return handler(ctx, req)
	}
}

// wrappedServerStream lets us override Context() on an incoming stream so
// downstream handlers (e.g. SubscribeToChat) see the JWT-verified user_id,
// mirroring what authUnaryInterceptor does for unary calls.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

func (s *GatewayServer) authStreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if _, ok := publicMethods[info.FullMethod]; ok {
			return handler(srv, stream)
		}

		token, ok := bearerTokenFromContext(stream.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "authorization bearer token missing")
		}

		res, err := s.AuthClient.ValidateToken(stream.Context(), &authpb.ValidateTokenRequest{
			Token: token,
		})
		if err != nil || !res.GetIsValid() || res.GetUserId() == "" {
			return status.Error(codes.Unauthenticated, "invalid or expired authorization token")
		}

		ctx := context.WithValue(stream.Context(), userIDContextKey, res.GetUserId())

		return handler(srv, &wrappedServerStream{ServerStream: stream, ctx: ctx})
	}
}

func bearerTokenFromContext(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", false
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return "", false
	}

	for _, value := range values {
		token := strings.TrimSpace(value)
		if token == "" {
			continue
		}

		parts := strings.Fields(token)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1], true
		}
	}

	return "", false
}
