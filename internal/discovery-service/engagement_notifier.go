package discovery

import (
	"context"
	"fmt"
	"log"
	"time"

	trackdb "github.com/fanyicharllson/phonkdrift-backend/internal/track-service/repository/db"
	authpb "github.com/fanyicharllson/phonkdrift-backend/pb/auth"
)

// EngagementNotifier periodically recommends a random track from the
// existing catalog to bring users back, independent of whether any new
// content was recently discovered — unlike TrendingNotifier, which only
// fires when something genuinely new is waiting to be announced.
type EngagementNotifier struct {
	repo       trackdb.Querier
	authClient authpb.AuthServiceClient
	interval   time.Duration
}

func NewEngagementNotifier(repo trackdb.Querier, authClient authpb.AuthServiceClient, interval time.Duration) *EngagementNotifier {
	return &EngagementNotifier{repo: repo, authClient: authClient, interval: interval}
}

// Start blocks — run it in its own goroutine (`go notifier.Start(ctx)`).
func (n *EngagementNotifier) Start(ctx context.Context) {
	log.Printf("🎧 Engagement notifier started — recommending every %s", n.interval)

	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	// Deliberately no immediate fire on startup (unlike TrendingNotifier) —
	// this runs purely on its own cadence so a pod restart doesn't trigger
	// an extra recommendation on top of the regular schedule.
	for {
		select {
		case <-ticker.C:
			n.recommendOne(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (n *EngagementNotifier) recommendOne(ctx context.Context) {
	tracks, err := n.repo.GetForYouTracks(ctx, 1)
	if err != nil {
		log.Printf("⚠️ Engagement notifier: failed to pick a track: %v", err)
		return
	}
	if len(tracks) == 0 {
		return
	}
	track := tracks[0]

	_, err = n.authClient.SendPushNotification(ctx, &authpb.PushNotificationRequest{
		Title:        "🎧 Haven't listened in a while?",
		Body:         fmt.Sprintf("Check out: %s — %s", track.ArtistName, track.Title),
		TargetUserId: "",
		DataType:     "engagement",
		DataId:       track.ID,
	})
	if err != nil {
		log.Printf("⚠️ Failed to send engagement push: %v", err)
		return
	}

	log.Printf("✅ Engagement push sent: %s — %s", track.ArtistName, track.Title)
}
