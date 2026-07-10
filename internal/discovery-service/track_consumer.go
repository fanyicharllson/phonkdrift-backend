package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"

	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	"github.com/rabbitmq/amqp091-go"
)

// TrackDiscoveredEvent mirrors track-service's repository.TrackDiscoveredEvent.
type TrackDiscoveredEvent struct {
	TrackID   string `json:"track_id"`
	YoutubeID string `json:"youtube_id"`
}

// TrackDownloadWorker consumes track.discovered events (published when a user
// favorites/adds-to-playlist a track that only existed as a YouTube search
// result) and downloads+uploads the audio in the background, reusing the
// exact same Uploader.DownloadAndUpload path the scheduled discovery cycle uses.
type TrackDownloadWorker struct {
	amqpChan *amqp091.Channel
	uploader *Uploader
	repo     trackdb.Querier
}

func NewTrackDownloadWorker(ch *amqp091.Channel, uploader *Uploader, repo trackdb.Querier) *TrackDownloadWorker {
	return &TrackDownloadWorker{amqpChan: ch, uploader: uploader, repo: repo}
}

func (w *TrackDownloadWorker) Start() {
	if err := w.amqpChan.ExchangeDeclare("track.events", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("Critical: Track download worker exchange declaration failed: %v", err)
	}

	q, err := w.amqpChan.QueueDeclare("discovery.track_events_queue", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Track download worker queue declaration failed: %v", err)
	}

	if err := w.amqpChan.QueueBind(q.Name, "track.discovered", "track.events", false, nil); err != nil {
		log.Fatalf("Critical: Failed to bind track download worker queue: %v", err)
	}

	msgs, err := w.amqpChan.Consume(q.Name, "discovery-service-track-consumer", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("Critical: Track download worker subscription failed: %v", err)
	}

	go func() {
		log.Println("Track Download Worker active via Topic Exchange. Polling... ⬇️")
		for d := range msgs {
			var event TrackDiscoveredEvent
			if err := json.Unmarshal(d.Body, &event); err != nil {
				log.Printf("⚠️ Failed to decode track.discovered event: %v", err)
				continue
			}
			w.handleTrackDiscovered(event)
		}
	}()
}

func (w *TrackDownloadWorker) handleTrackDiscovered(event TrackDiscoveredEvent) {
	ctx := context.Background()

	cdnURL, err := w.uploader.DownloadAndUpload(ctx, event.YoutubeID)
	if err != nil {
		// Non-fatal: the track stays fully streamable via the yt-dlp -g fallback
		// in GetAudioStream even if this background download never succeeds.
		log.Printf("⚠️ Background download failed for track %s (youtube_id=%s): %v", event.TrackID, event.YoutubeID, err)
		return
	}

	if err := w.repo.UpdateTrackStorageURL(ctx, trackdb.UpdateTrackStorageURLParams{
		ID:         event.TrackID,
		StorageUrl: sql.NullString{String: cdnURL, Valid: true},
	}); err != nil {
		log.Printf("⚠️ Failed to save storage_url for track %s: %v", event.TrackID, err)
		return
	}

	log.Printf("✅ Background download complete for track %s -> %s", event.TrackID, cdnURL)
}
