package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const youtubeSearchURL = "https://www.googleapis.com/youtube/v3/search"

type YouTubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumbnails  struct {
				High struct {
					URL string `json:"url"`
				} `json:"high"`
			} `json:"thumbnails"`
			ChannelTitle string `json:"channelTitle"`
		} `json:"snippet"`
	} `json:"items"`
}

type YouTubeVideoResponse struct {
	Items []struct {
		ID             string `json:"id"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Statistics struct {
			ViewCount string `json:"viewCount"`
		} `json:"statistics"`
	} `json:"items"`
}

type DiscoveredTrack struct {
	YoutubeID    string
	Title        string
	ArtistName   string
	ThumbnailURL string
	Duration     string
	ViewCount    int64
}

type Worker struct {
	apiKey     string
	httpClient *http.Client
	queries    []string
}

func NewWorker(apiKey string) *Worker {
	return &Worker{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		queries: []string{
			"Montagem funks slowed 2026",
			"Trending Phonk 2026",
		},
	}
}

func (w *Worker) SearchPhonkTracks(ctx context.Context) ([]DiscoveredTrack, error) {
	var allTracks []DiscoveredTrack
	seen := map[string]bool{}

	for _, query := range w.queries {
		tracks, err := w.searchQuery(ctx, query)
		if err != nil {
			log.Printf("⚠️ Query '%s' failed: %v", query, err)
			continue
		}

		for _, t := range tracks {
			if !seen[t.YoutubeID] {
				seen[t.YoutubeID] = true
				allTracks = append(allTracks, t)
			}
		}

		// Respect YouTube API rate limits
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("🎵 Discovery complete: found %d unique tracks", len(allTracks))
	return allTracks, nil
}

func (w *Worker) searchQuery(ctx context.Context, query string) ([]DiscoveredTrack, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", youtubeSearchURL, nil)
	q := req.URL.Query()
	q.Add("part", "snippet")
	q.Add("q", query)
	q.Add("type", "video")
	q.Add("videoCategoryId", "10") // Music category
	q.Add("maxResults", "10")
	q.Add("order", "viewCount")
	q.Add("videoDuration", "short") // Under 4 minutes — phonk tracks
	q.Add("key", w.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var searchResp YouTubeSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	// Collect video IDs for details fetch
	var videoIDs []string
	snippetMap := map[string]struct {
		Title        string
		ChannelTitle string
		ThumbnailURL string
	}{}

	for _, item := range searchResp.Items {
		if item.ID.VideoID == "" {
			continue
		}
		videoIDs = append(videoIDs, item.ID.VideoID)
		snippetMap[item.ID.VideoID] = struct {
			Title        string
			ChannelTitle string
			ThumbnailURL string
		}{
			Title:        item.Snippet.Title,
			ChannelTitle: item.Snippet.ChannelTitle,
			ThumbnailURL: item.Snippet.Thumbnails.High.URL,
		}
	}

	if len(videoIDs) == 0 {
		return nil, nil
	}

	// Fetch video details (duration + view count)
	details, err := w.fetchVideoDetails(ctx, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch video details: %w", err)
	}

	var tracks []DiscoveredTrack
	for _, detail := range details {
		snippet, ok := snippetMap[detail.ID]
		if !ok {
			continue
		}

		// Filter: skip videos with less than 50k views
		var viewCount int64
		fmt.Sscanf(detail.Statistics.ViewCount, "%d", &viewCount)
		if viewCount < 50000 {
			continue
		}

		tracks = append(tracks, DiscoveredTrack{
			YoutubeID:    detail.ID,
			Title:        snippet.Title,
			ArtistName:   snippet.ChannelTitle,
			ThumbnailURL: snippet.ThumbnailURL,
			Duration:     parseDuration(detail.ContentDetails.Duration),
			ViewCount:    viewCount,
		})
	}

	return tracks, nil
}

func (w *Worker) fetchVideoDetails(ctx context.Context, videoIDs []string) ([]struct {
	ID             string `json:"id"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	Statistics struct {
		ViewCount string `json:"viewCount"`
	} `json:"statistics"`
}, error) {
	ids := ""
	for i, id := range videoIDs {
		if i > 0 {
			ids += ","
		}
		ids += id
	}

	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://www.googleapis.com/youtube/v3/videos", nil)
	q := req.URL.Query()
	q.Add("part", "contentDetails,statistics")
	q.Add("id", ids)
	q.Add("key", w.apiKey)
	req.URL.RawQuery = q.Encode()

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result YouTubeVideoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// parseDuration converts ISO 8601 (PT2M45S) → "02:45"
func parseDuration(iso string) string {
	d, err := time.ParseDuration(
		fmt.Sprintf("%sm%ss",
			extractISO(iso, "M"),
			extractISO(iso, "S"),
		),
	)
	if err != nil {
		return "00:00"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", mins, secs)
}

func extractISO(iso, unit string) string {
	start := -1
	for i, c := range iso {
		if c == 'T' || c == 'P' {
			start = i
		}
		if string(c) == unit {
			for j := start + 1; j < i; j++ {
				if iso[j] >= '0' && iso[j] <= '9' {
					return iso[j : i]
				}
			}
		}
	}
	return "0"
}