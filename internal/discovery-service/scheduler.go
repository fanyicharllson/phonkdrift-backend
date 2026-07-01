package discovery

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"

	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	"github.com/google/uuid"
)

type Scheduler struct {
	worker   *Worker
	uploader *Uploader
	repo     trackdb.Querier
	interval time.Duration
}

func NewScheduler(worker *Worker, uploader *Uploader, repo trackdb.Querier, interval time.Duration) *Scheduler {
	return &Scheduler{
		worker:   worker,
		uploader: uploader,
		repo:     repo,
		interval: interval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("🔄 Discovery scheduler started — running every %s", s.interval)

	// Run immediately on startup
	s.runCycle(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runCycle(ctx)
		case <-ctx.Done():
			log.Println("🛑 Discovery scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) runCycle(ctx context.Context) {
	log.Println("🎵 Starting phonk discovery cycle...")

	tracks, err := s.worker.SearchPhonkTracks(ctx)
	if err != nil {
		log.Printf("❌ Discovery cycle failed: %v", err)
		return
	}

	newCount := 0
	for _, track := range tracks {
		// Skip if already in DB
		existing, _ := s.repo.GetTrackByYoutubeID(ctx, track.YoutubeID)
		if existing.YoutubeID == track.YoutubeID {
			continue
		}

		// Download + upload to DO Spaces
		storageURL, err := s.uploader.DownloadAndUpload(ctx, track.YoutubeID)
		if err != nil {
			log.Printf("⚠️  Failed to process track %s: %v", track.YoutubeID, err)
			continue
		}

		// Save to DB — auto-approved since it's our own pipeline
		_, err = s.repo.InsertTrack(ctx, trackdb.InsertTrackParams{
			ID:           uuid.New().String(),
			Title:        track.Title,
			ArtistID:     "system",
			ArtistName:   track.ArtistName,
			Duration:     track.Duration,
			ThumbnailUrl: track.ThumbnailURL,
			YoutubeID:    track.YoutubeID,
			StorageUrl:   sql.NullString{String: storageURL, Valid: storageURL != ""},
			Genre:        sql.NullString{String: detectGenre(track.Title), Valid: true},
			Source:       "auto",
			IsApproved:   true,
			YtViewCount:  sql.NullInt64{Int64: track.ViewCount, Valid: true},
		})
		if err != nil {
			log.Printf("⚠️  DB insert failed for %s: %v", track.YoutubeID, err)
			continue
		}

		newCount++
		log.Printf("✅ Added: %s — %s", track.ArtistName, track.Title)

		// Small delay between downloads to avoid IP bans
		time.Sleep(3 * time.Second)
	}

	log.Printf("🏁 Discovery cycle done. Added %d new tracks.", newCount)
}

func (s *Scheduler) RunOnce(ctx context.Context) {
	s.runCycle(ctx)
}

// detectGenre classifies phonk subgenre from title keywords
func detectGenre(title string) string {
	title = strings.ToLower(title)
	switch {
	case strings.Contains(title, "drift"):
		return "drift_phonk"
	case strings.Contains(title, "dark"):
		return "dark_phonk"
	case strings.Contains(title, "brazilian") || strings.Contains(title, "brasil"):
		return "brazilian_phonk"
	case strings.Contains(title, "aggressive"):
		return "aggressive_phonk"
	case strings.Contains(title, "underground"):
		return "underground_phonk"
	default:
		return "phonk"
	}
}