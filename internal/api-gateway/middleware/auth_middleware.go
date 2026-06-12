package middleware

import (
	"context"
	"net/http"
	"strings"

	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthRequired intercepts incoming REST requests, extracts the JWT,
// and calls the internal Auth Service via gRPC to validate the session.
func AuthRequired(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenStr string

		//  Extract from Authorization Header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			splitToken := strings.Split(authHeader, "Bearer ")
			if len(splitToken) == 2 {
				tokenStr = strings.TrimSpace(splitToken[1])
			}
		}

		// : Fallback to reading httpOnly cookie if header is empty
		if tokenStr == "" {
			if cookie, err := c.Cookie("access_token"); err == nil {
				tokenStr = cookie
			}
		}

		// Security Guard: If no token was found in either layer, reject instantly
		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication token missing or malformed"})
			c.Abort()
			return
		}

		//  INTERNAL RPC CALL: Ask Auth Microservice if this token is actually valid
		res, err := authClient.ValidateToken(context.Background(), &authpb.ValidateTokenRequest{
			Token: tokenStr,
		})

		if err != nil {
			// Convert raw gRPC error statuses cleanly into standard HTTP codes
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Unauthenticated {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired or invalid credentials"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication security subsystem offline"})
			}
			c.Abort()
			return
		}

		if !res.GetIsValid() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized token signature mismatch"})
			c.Abort()
			return
		}

		// Context Injections: Pass the verified parameters down to Tracking/Chat services
		c.Set("user_id", res.GetUserId())
		c.Set("username", res.GetUsername())

		// Continue the lifecycle chain execution safely
		c.Next()
	}
}
