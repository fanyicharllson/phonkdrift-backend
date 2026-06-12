package http

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/config"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	"github.com/gin-gonic/gin"
)

func StartHTTPServer(cfg *config.Config, authClient authpb.AuthServiceClient) {
	r := gin.Default()

	// Public Auth Delivery Group
	publicAuth := r.Group("/api/v1/auth")
	{
		publicAuth.POST("/register", handleRegister(authClient))
		publicAuth.POST("/login", handleLogin(authClient))
		publicAuth.POST("/verify-code", handleVerifyCode(authClient))
	}

	log.Printf("HTTP REST API Gateway running on port %s 🏎️💨", cfg.HTTPPort)
	if err := r.Run(cfg.HTTPPort); err != nil {
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