package http

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fanyicharllson/phonkdrift-backend/internal/api-gateway/middleware"
	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	discovery "github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxAvatarBytes = 5 << 20 // 5MB

var allowedAvatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

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

	// CORS must be registered before any route groups so it applies to all of them,
	// including preflight OPTIONS requests for routes with no explicit OPTIONS handler
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// 🔑 Public Auth Delivery Group
	publicAuth := r.Group("/api/v1/auth")
	{
		publicAuth.POST("/register", handleRegister(authClient))
		publicAuth.POST("/login", handleLogin(authClient))
		publicAuth.POST("/verify-code", handleVerifyCode(authClient))
        publicAuth.GET("/user/status/:user_id", handleGetUserStatus(authClient))
	}

	// 🏎️ Track Engine Delivery Group (requires validated JWT session)
	publicTrack := r.Group("/api/v1/tracks")
	publicTrack.Use(middleware.AuthRequired(authClient))
	{
		publicTrack.GET("/liked", handleGetLikedTracks(trackClient))
		publicTrack.GET("/trending", handleGetTrendingTracks(trackClient))
	}

	// 👤 Authenticated Profile/User Delivery Group
	authedUsers := r.Group("/api/v1/users")
	authedUsers.Use(middleware.AuthRequired(authClient))
	{
		authedUsers.POST("/me/avatar", handleUploadAvatar(authClient, uploader))
		authedUsers.PATCH("/me/username", handleUpdateUsername(authClient))
		authedUsers.PATCH("/me/password", handleChangePassword(authClient))
		authedUsers.PATCH("/me/phonk-level", handleUpdatePhonkLevel(authClient))
		authedUsers.POST("/me/feedback", handleSubmitFeedback(authClient))
	}

	// 💬 Chat media upload (binary upload only — sending the message itself is a gRPC call)
	chatMedia := r.Group("/api/v1/chat")
	chatMedia.Use(middleware.AuthRequired(authClient))
	{
		chatMedia.POST("/media", handleUploadChatMedia(uploader))
	}

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

func handleUploadAvatar(authClient authpb.AuthServiceClient, uploader *discovery.Uploader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uploader == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "avatar storage is not configured"})
			return
		}

		userID := c.GetString("user_id")

		fileHeader, err := c.FormFile("avatar")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "avatar file is required"})
			return
		}
		if fileHeader.Size > maxAvatarBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "avatar must be 5MB or smaller"})
			return
		}

		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
			return
		}

		contentType := http.DetectContentType(data)
		if !allowedAvatarTypes[contentType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "avatar must be a JPEG, PNG, or WebP image"})
			return
		}

		ext := ".jpg"
		switch contentType {
		case "image/png":
			ext = ".png"
		case "image/webp":
			ext = ".webp"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cdnURL, err := uploader.PutObject(ctx, "avatars/"+userID+ext, data, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload avatar: " + err.Error()})
			return
		}

		if _, err := authClient.UploadAvatar(ctx, &authpb.UploadAvatarRequest{
			UserId:    userID,
			AvatarUrl: cdnURL,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"avatar_url": cdnURL})
	}
}

func handleUpdateUsername(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.UpdateUsername(ctx, &authpb.UpdateUsernameRequest{
			UserId:      c.GetString("user_id"),
			NewUsername: req.Username,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.AlreadyExists {
				c.JSON(http.StatusConflict, gin.H{"error": st.Message()})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": res.GetSuccess(), "user": res.GetUser()})
	}
}

func handleChangePassword(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			OldPassword string `json:"old_password" binding:"required"`
			NewPassword string `json:"new_password" binding:"required,min=6"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.ChangePassword(ctx, &authpb.ChangePasswordRequest{
			UserId:      c.GetString("user_id"),
			OldPassword: req.OldPassword,
			NewPassword: req.NewPassword,
		})
		if err != nil {
			st, _ := status.FromError(err)
			if st.Code() == codes.PermissionDenied {
				c.JSON(http.StatusForbidden, gin.H{"error": st.Message()})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": res.GetSuccess(), "message": res.GetMessage()})
	}
}

func handleUpdatePhonkLevel(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PhonkLevel string `json:"phonk_level" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.UpdateProfile(ctx, &authpb.UpdateProfileRequest{
			UserId:     c.GetString("user_id"),
			PhonkLevel: req.PhonkLevel,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": res.GetSuccess(), "user": res.GetUser()})
	}
}

func handleSubmitFeedback(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Rating     int32  `json:"rating" binding:"required,min=1,max=5"`
			Comment    string `json:"comment"`
			AppVersion string `json:"app_version"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := authClient.SubmitFeedback(ctx, &authpb.SubmitFeedbackRequest{
			UserId:     c.GetString("user_id"),
			Rating:     req.Rating,
			Comment:    req.Comment,
			AppVersion: req.AppVersion,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": res.GetSuccess(), "feedback_id": res.GetFeedbackId()})
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