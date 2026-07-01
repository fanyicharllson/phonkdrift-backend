package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	discovery "github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/gin-gonic/gin"
)

// StartHTTPServer accepts BOTH clients to multiplex proxy endpoints out cleanly
func StartHTTPServer(
	cfg *config.Config,
	authClient authpb.AuthServiceClient,
	trackClient trackpb.TrackServiceClient,
	uploader *discovery.Uploader,
	scheduler *discovery.Scheduler,
) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// security proxy warning right after initialization
	r.SetTrustedProxies(nil)

	// 🔑 Public Auth Delivery Group
	publicAuth := r.Group("/api/v1/auth")
	{
		publicAuth.POST("/register", handleRegister(authClient))
		publicAuth.POST("/login", handleLogin(authClient))
		publicAuth.POST("/verify-code", handleVerifyCode(authClient))
        publicAuth.GET("/user/status/:user_id", handleGetUserStatus(authClient))
	}

	// 🏎️ Public Track Engine Delivery Group (Placeholder for rest testing if needed)
	// publicTrack := r.Group("/api/v1/tracks")
	// {
	//     // You can add HTTP mappings to proxy into trackClient here later!
	// }

	// Register Admin Routes
	RegisterAdminRoutes(r, cfg, authClient, trackClient, uploader, scheduler)

	// Format port address cleanly using unified config structure
	address := ":" + cfg.ApiGatewayHttpPort
	log.Printf("HTTP REST API Gateway running on port %s 🏎️💨", address)
	if err := r.Run(address); err != nil {
		log.Fatalf("Failed to run HTTP server: %v", err)
	}
}

func handleRegister(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required,min=6"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.RegisterUser(ctx, &authpb.RegisterRequest{
			Username: req.Username,
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user_id": res.GetUserId(), "message": res.GetMessage()})
	}
}

func handleLogin(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email" binding:"required,email"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.LoginUser(ctx, &authpb.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":      res.GetToken(),
			"user_id":    res.GetUserId(),
			"expires_at": res.GetExpiresAt(),
		})
	}
}

func handleGetUserStatus(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		if userID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user_id required"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.GetUserStatus(ctx, &authpb.GetUserStatusRequest{
			UserId: userID,
		})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"is_banned":   res.GetIsBanned(),
			"ban_reason":  res.GetBanReason(),
			"username":    res.GetUsername(),
			"phonk_level": res.GetPhonkLevel(),
		})
	}
}

func handleVerifyCode(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email string `json:"email" binding:"required,email"`
			Code  string `json:"code" binding:"required,len=6"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.VerifyCode(ctx, &authpb.VerifyRequest{
			Email: req.Email,
			Code:  req.Code,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":    res.GetSuccess(),
			"message":    res.GetMessage(),
			"token":      res.GetToken(),
			"expires_at": res.GetExpiresAt(),
			"user_id":    res.GetUserId(),
		})
	}
}