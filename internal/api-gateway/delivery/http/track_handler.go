package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	trackpb "github.com/fanyicharllson/phonkdrift-backend/pb/track"
	"github.com/gin-gonic/gin"
)

// ─── PUBLIC TRACK HANDLERS ───────────────────────────────────────────────────

func handleGetLikedTracks(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		res, err := trackClient.GetLikedTracks(ctx, &trackpb.GetLikedTracksRequest{
			UserId: userID,
			Limit:  int32(limit),
			Page:   int32(page),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"tracks": res.GetTracks(),
			"total":  res.GetTotal(),
		})
	}
}

func handleGetTrendingTracks(trackClient trackpb.TrackServiceClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		res, err := trackClient.GetTrendingTracks(ctx, &trackpb.TrendingRequest{
			Limit: int32(limit),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"tracks": res.GetTracks(),
		})
	}
}
