package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2/google"
)

const fcmV1URL = "https://fcm.googleapis.com/v1/projects/%s/messages:send"

func sendFCMNotifications(tokens []string, title, body, dataType, dataID string) (int, error) {
	projectID, accessToken, err := getFCMAccessToken()
	if err != nil {
		return 0, fmt.Errorf("FCM auth failed: %w", err)
	}

	url := fmt.Sprintf(fcmV1URL, projectID)
	client := &http.Client{Timeout: 10 * time.Second}
	sentCount := 0

	for _, token := range tokens {
		payload := map[string]interface{}{
			"message": map[string]interface{}{
				"token": token,
				"notification": map[string]string{
					"title": title,
					"body":  body,
				},
				"data": map[string]string{
					"type": dataType,
					"id":   dataID,
				},
				"android": map[string]interface{}{
					"priority": "high",
				},
				"apns": map[string]interface{}{
					"payload": map[string]interface{}{
						"aps": map[string]interface{}{
							"sound": "default",
						},
					},
				},
			},
		}

		jsonBody, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			log.Printf("⚠️ FCM request build failed for token %s: %v", token[:10], err)
			continue
		}

		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠️ FCM send failed for token %s: %v", token[:10], err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			sentCount++
		} else {
			log.Printf("⚠️ FCM non-200 for token %s: %d", token[:10], resp.StatusCode)
		}
	}

	log.Printf("📱 FCM: sent %d/%d notifications", sentCount, len(tokens))
	return sentCount, nil
}

func getFCMAccessToken() (string, string, error) {
	serviceAccountJSON := os.Getenv("FCM_SERVICE_ACCOUNT_JSON")
	if serviceAccountJSON == "" {
		return "", "", fmt.Errorf("FCM_SERVICE_ACCOUNT_JSON not set")
	}

	// Decode base64 if needed
	jsonBytes := []byte(serviceAccountJSON)

	conf, err := google.CredentialsFromJSON(
		context.Background(),
		jsonBytes,
		"https://www.googleapis.com/auth/firebase.messaging",
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse service account: %w", err)
	}

	// Extract project ID from JSON
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	json.Unmarshal(jsonBytes, &sa)

	token, err := conf.TokenSource.Token()
	if err != nil {
		return "", "", fmt.Errorf("failed to get OAuth2 token: %w", err)
	}

	return sa.ProjectID, token.AccessToken, nil
}
