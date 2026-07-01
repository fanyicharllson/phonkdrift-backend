package http

import (
	"context"
	"net/http"
	"strconv"
	"time"
	"strings"

	discovery "github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	"github.com/fanyicharllson/phonkdrift-backend/internal/config"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(
	r *gin.Engine,
	cfg *config.Config,
	authClient authpb.AuthServiceClient,
	trackClient trackpb.TrackServiceClient,
	uploader *discovery.Uploader,
	scheduler *discovery.Scheduler,
) {
	admin := r.Group("/api/v1/admin")
	admin.Use(adminAuthMiddleware(cfg.AdminJWTSecret))
	{
		// Track management
		admin.POST("/tracks/seed", handleSeedTrack(uploader, trackClient, cfg))
		admin.GET("/tracks", handleListTracks(trackClient))
		admin.PUT("/tracks/:id/approve", handleApproveTrack(trackClient))
		admin.PUT("/tracks/:id/reject", handleRejectTrack(trackClient))
		admin.PUT("/tracks/:id/feature", handleFeatureTrack(trackClient))
		admin.DELETE("/tracks/:id", handleDeleteTrack(trackClient))

		// User management
		admin.POST("/users/:id/ban", handleBanUser(authClient))
		admin.POST("/users/:id/unban", handleUnbanUser(authClient))

		// Notifications
		admin.POST("/notifications/send", handleSendNotification(authClient, cfg))

		// Stats
		admin.GET("/stats", handleGetStats(trackClient, authClient))

		// Manual discovery trigger
		admin.POST("/discovery/run", handleRunDiscovery(scheduler))
	}
}

// ─── ADMIN AUTH MIDDLEWARE ───────────────────────────────────────────────────

func adminAuthMiddleware(adminSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" || token != "Bearer "+adminSecret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin access denied"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ─── TRACK HANDLERS ──────────────────────────────────────────────────────────

func handleSeedTrack(
	uploader *discovery.Uploader,
	trackClient trackpb.TrackServiceClient,
	cfg *config.Config,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			YoutubeURL   string `json:"youtube_url" binding:"required"`
			Title        string `json:"title" binding:"required"`
			ArtistName   string `json:"artist_name" binding:"required"`
			Genre        string `json:"genre"`
			ThumbnailURL string `json:"thumbnail_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Extract YouTube ID from URL
		youtubeID := extractYoutubeID(req.YoutubeURL)
		if youtubeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid YouTube URL"})
			return
		}

		// Download + upload in background
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		storageURL, err := uploader.DownloadAndUpload(ctx, youtubeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Download failed: " + err.Error()})
			return
		}

		genre := req.Genre
		if genre == "" {
			genre = "phonk"
		}

		// Save via track service gRPC
		res, err := trackClient.SeedTrack(ctx, &trackpb.SeedTrackRequest{
			YoutubeId:    youtubeID,
			Title:        req.Title,
			ArtistName:   req.ArtistName,
			Genre:        genre,
			ThumbnailUrl: req.ThumbnailURL,
			StorageUrl:   storageURL,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":     "Track seeded successfully",
			"track_id":    res.GetTrackId(),
			"storage_url": storageURL,
		})
	}
}

func handleListTracks(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		res, err := trackClient.ListTracksAdmin(ctx, &trackpb.ListTracksAdminRequest{
			Page:  int32(page),
			Limit: int32(limit),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, res)
	}
}

func handleApproveTrack(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := trackClient.ApproveTrack(ctx, &trackpb.TrackActionRequest{TrackId: id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Track approved"})
	}
}

func handleRejectTrack(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := trackClient.RejectTrack(ctx, &trackpb.TrackActionRequest{TrackId: id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Track rejected"})
	}
}

func handleFeatureTrack(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Featured bool `json:"featured"`
		}
		c.ShouldBindJSON(&req)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := trackClient.FeatureTrack(ctx, &trackpb.FeatureTrackRequest{
			TrackId:    id,
			IsFeatured: req.Featured,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Track feature status updated"})
	}
}

func handleDeleteTrack(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := trackClient.DeleteTrack(ctx, &trackpb.TrackActionRequest{TrackId: id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Track deleted"})
	}
}

// ─── USER HANDLERS ───────────────────────────────────────────────────────────

func handleBanUser(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Reason string `json:"reason"`
		}
		c.ShouldBindJSON(&req)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := authClient.BanUser(ctx, &authpb.BanUserRequest{
			UserId: id,
			Reason: req.Reason,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "User banned"})
	}
}

func handleUnbanUser(authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := authClient.UnbanUser(ctx, &authpb.UnbanUserRequest{UserId: id})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "User unbanned"})
	}
}

// ─── NOTIFICATION HANDLER ────────────────────────────────────────────────────

func handleSendNotification(authClient authpb.AuthServiceClient, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title        string `json:"title" binding:"required"`
			Body         string `json:"body" binding:"required"`
			TargetUserID string `json:"target_user_id"` // empty = broadcast to all
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
			Title:        req.Title,
			Body:         req.Body,
			TargetUserId: req.TargetUserID,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Notification sent"})
	}
}

// ─── STATS HANDLER ───────────────────────────────────────────────────────────

func handleGetStats(trackClient trackpb.TrackServiceClient, authClient authpb.AuthServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stats, err := trackClient.GetAdminStats(ctx, &trackpb.Empty{})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, stats)
	}
}

// ─── MANUAL DISCOVERY TRIGGER ────────────────────────────────────────────────

func handleRunDiscovery(scheduler *discovery.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		go scheduler.RunOnce(context.Background())
		c.JSON(http.StatusOK, gin.H{"message": "Discovery cycle triggered"})
	}
}

// ─── HELPERS ─────────────────────────────────────────────────────────────────

func extractYoutubeID(url string) string {
	// Handle formats:
	// https://www.youtube.com/watch?v=dQw4w9WgXcQ
	// https://youtu.be/dQw4w9WgXcQ
	if strings.Contains(url, "v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			id := strings.Split(parts[1], "&")[0]
			return id
		}
	}
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			return strings.Split(parts[1], "?")[0]
		}
	}
	return ""
}