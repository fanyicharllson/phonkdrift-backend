package http

import (
	"context"
	"io"
	"net/http"
	"time"

	discovery "github.com/fanyicharllson/phonkdrift-backend/internal/discovery-service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxChatAudioBytes = 10 << 20 // 10MB

var allowedChatAudioTypes = map[string]bool{
	"audio/mpeg":  true,
	"audio/mp4":   true,
	"audio/wav":   true,
	"audio/x-wav": true,
	"audio/ogg":   true,
	"audio/aac":   true,
	"audio/webm":  true,
}

// handleUploadChatMedia only uploads the audio clip to DO Spaces and returns
// its URL — sending the actual message (with this media_url, an optional
// caption, and an optional reply_to_id) happens via the SendMessage gRPC RPC,
// same "REST for the binary, gRPC for the action" split used for avatars.
func handleUploadChatMedia(uploader *discovery.Uploader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if uploader == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "media storage is not configured"})
			return
		}

		userID := c.GetString("user_id")

		fileHeader, err := c.FormFile("audio")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "audio file is required"})
			return
		}
		if fileHeader.Size > maxChatAudioBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "audio clip must be 10MB or smaller"})
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
		if !allowedChatAudioTypes[contentType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported audio format"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		key := "chat_media/" + userID + "/" + uuid.New().String()
		mediaURL, err := uploader.PutObject(ctx, key, data, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload audio: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"media_url": mediaURL})
	}
}
