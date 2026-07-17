package http

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// versionInfo mirrors version.json — the OTA update manifest.
type versionInfo struct {
	LatestVersion string `json:"latest_version"`
	ApkURL        string `json:"apk_url"`
	Mandatory     bool   `json:"mandatory"`
}

// handleCheckUpdate reads version.json fresh from disk on every request — no
// in-memory caching — so operators can update it in place (a mounted k8s
// ConfigMap in production, or the local file in dev) without rebuilding the
// image or restarting the pod.
func handleCheckUpdate(versionFilePath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := versionFilePath
		if path == "" {
			path = "version.json" // relative to the working directory (container WORKDIR /app)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "version info not available"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read version info"})
			return
		}

		var info versionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "version info is malformed"})
			return
		}

		c.JSON(http.StatusOK, info)
	}
}
